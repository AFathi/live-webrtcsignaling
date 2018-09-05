package dtls

//#include "shim.h"
import "C"

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"runtime"
	"strings"
	"unsafe"

	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
	"github.com/heytribe/live-webrtcsignaling/packet"
)

const dtlsCiphers = "ALL:NULL:eNULL:aNULL"

//const dtlsCiphers = "ALL:!ADH:!LOW:!EXP:!MD5:@STRENGTH"

var (
	localFingerprint   string
	libraryInitialized bool
)

var (
	tryAgain = errors.New("try again")
)

type DTLSSession struct {
	*SSL
	ch               chan *packet.UDP
	rAddr            *net.UDPAddr
	ctx              *Ctx // for gc
	intoOSSL         *readBio
	fromOSSL         *writeBio
	isShutdown       bool
	mutex            my.Mutex
	want_read_future *Future
}

var log plogger.PLogger

func init() {
	log = plogger.New().Prefix("DTLS")
	log.Infof("Initialization")
	if rc := C.shimInit(); rc != 0 {
		panic(fmt.Errorf("c_shimInit() failed with return code %d", rc))
	}
}

func GetLocalFingerprint() string {
	return localFingerprint
}

func verifyCallback(preverifyOk bool, store *CertificateStoreCtx) bool {
	log.Infof("preverifyOk is %v, store is %#v", preverifyOk, store)

	return true
}

func infoCallback(s *C.SSL, where int, ret int) {
	var str string
	var w int

	w = where &^ int(C.SSL_ST_MASK)

	if w&int(C.SSL_CB_ALERT) == 0 {
		return
	}

	if w&int(C.SSL_ST_CONNECT) > 0 {
		str = "SSL_connect"
	} else {
		if w&int(C.SSL_ST_ACCEPT) > 0 {
			str = "SSL_accept"
		} else {
			str = "undefined"
		}
	}

	if where&int(C.SSL_CB_LOOP) > 0 {
		log.Infof("%s:%s", str, C.GoString(C.SSL_state_string_long(s)))
	} else {
		if where&int(C.SSL_CB_ALERT) > 0 {
			if where&int(C.SSL_CB_READ) > 0 {
				str = "read"
			} else {
				str = "write"
			}
			log.Infof("alert %s:%s:%s", str, C.GoString(C.SSL_alert_type_string_long(C.int(ret))), C.GoString(C.SSL_alert_desc_string_long(C.int(ret))))
		} else {
			if where&int(C.SSL_CB_EXIT) > 0 {
				if ret == 0 {
					log.Infof("%s:failed in %s", str, C.GoString(C.SSL_state_string_long(s)))
				} else {
					if ret < 0 {
						log.Infof("%s:error in %s", str, C.GoString(C.SSL_state_string_long(s)))
					}
				}
			}
		}
	}

	return
}

func LoadKeys(ctx *Ctx, certFile string, keyFile string) (cert *Certificate, key PrivateKey, err error) {
	var certBytes []byte

	certBytes, err = ioutil.ReadFile(certFile)
	if err != nil {
		return
	}

	certs := SplitPEM(certBytes)
	if len(certs) == 0 {
		err = fmt.Errorf("No PEM certificate found in '%s'", certFile)
		return
	}

	first, certs := certs[0], certs[1:]
	cert, err = LoadCertificateFromPEM(first)
	if err != nil {
		return
	}

	err = ctx.UseCertificate(cert)
	if err != nil {
		return
	}

	var c *Certificate
	for _, pem := range certs {
		c, err = LoadCertificateFromPEM(pem)
		if err != nil {
			return
		}
		err = ctx.AddChainCertificate(c)
		if err != nil {
			return
		}
	}

	var keyBytes []byte
	keyBytes, err = ioutil.ReadFile(keyFile)
	if err != nil {
		return
	}

	key, err = LoadPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return
	}

	err = ctx.UsePrivateKey(key)
	if err != nil {
		return
	}

	return
}

func Init(certFilePath string, keyFilePath string) (ctx *Ctx, err error) {
	if libraryInitialized == true {
		err = errors.New("DTLS library is already initialized")
		return
	}

	ctx, err = NewCtxWithVersion(DTLSv1)
	if err != nil {
		err = errors.New("could not create new OpenSSL context with openssl.DTLS method")
		return
	}

	ctx.SetVerify(VerifyPeer|VerifyFailIfNoPeerCert, verifyCallback)
	ctx.SetTLSExtUseSrtp("SRTP_AES128_CM_SHA1_80")

	var certFile *Certificate
	//var keyFile openssl.PrivateKey
	certFile, _, err = LoadKeys(ctx, certFilePath, keyFilePath)
	if err != nil {
		return
	}
	err = ctx.SetCipherList(dtlsCiphers)
	if err != nil {
		return
	}
	var digest []byte
	digest, err = certFile.Digest(EVP_SHA256)
	if err != nil {
		return
	}
	log.Infof("digest is %#v", digest)

	strSlice := make([]string, len(digest))
	for i, b := range digest {
		strSlice[i] = fmt.Sprintf("%.2X", b)
	}
	localFingerprint = strings.Join(strSlice, ":")

	// No auto mtu
	ctx.SetOptions(OpNoQueryMtu)

	// Need to set ECDH group for Chrome
	ctx.SetOptions(OpSingleECDHUse)
	ctx.SetEllipticCurve(Prime256v1)

	libraryInitialized = true

	return
}

func newSSL(ctx *C.SSL_CTX) (*C.SSL, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ssl := C.SSL_new(ctx)
	if ssl == nil {
		return nil, errorFromErrorQueue()
	}

	return ssl, nil
}

func (dtlsSession *DTLSSession) Close() error {
	dtlsSession.mutex.Lock()
	if dtlsSession.isShutdown {
		dtlsSession.mutex.Unlock()
		return nil
	}
	dtlsSession.isShutdown = true
	dtlsSession.mutex.Unlock()
	return nil
}

type DtlsRole int

const (
	DtlsRoleClient DtlsRole = 0
	DtlsRoleServer DtlsRole = 1
)

func (ctx *Ctx) NewDTLS(ch chan *packet.UDP, rAddr *net.UDPAddr, dtlsRole DtlsRole) (*DTLSSession, error) {
	ssl, err := newSSL(ctx.ctx)
	if err != nil {
		return nil, err
	}

	intoOSSL := &readBio{}
	fromOSSL := &writeBio{}

	if ctx.GetMode()&ReleaseBuffers > 0 {
		intoOSSL.release_buffers = true
		fromOSSL.release_buffers = true
	}

	//intoOSSLBio := intoOSSL.MakeCBIO()
	intoOSSLBio := intoOSSL.MakeCBIO()
	fromOSSLBio := fromOSSL.MakeCBIO()
	if intoOSSLBio == nil || fromOSSLBio == nil {
		// these frees are null safe
		C.BIO_free(intoOSSLBio)
		C.BIO_free(fromOSSLBio)
		C.SSL_free(ssl)

		return nil, errors.New("failed to allocate memory BIO")
	}

	// the ssl object takes ownership of these objects now
	C.SSL_set_bio(ssl, intoOSSLBio, fromOSSLBio)
	C.X_SSL_set_mtu(ssl, C.int(1200))

	s := &SSL{ssl: ssl}
	C.SSL_set_ex_data(s.ssl, get_ssl_idx(), unsafe.Pointer(s))
	s.SetVerifyCallback(verifyCallback)
	s.SetInfoCallback(infoCallback)

	dtlsSession := &DTLSSession{
		SSL:      s,
		ch:       ch,
		rAddr:    rAddr,
		ctx:      ctx,
		intoOSSL: intoOSSL,
		fromOSSL: fromOSSL,
	}

	runtime.SetFinalizer(dtlsSession, func(dtlsSession *DTLSSession) {
		dtlsSession.intoOSSL.Disconnect(intoOSSLBio)
		dtlsSession.fromOSSL.Disconnect(fromOSSLBio)
		C.SSL_free(dtlsSession.ssl)
	})

	if dtlsRole == DtlsRoleClient {
		C.SSL_set_connect_state(dtlsSession.ssl)
	} else {
		C.SSL_set_accept_state(dtlsSession.ssl)
	}

	return dtlsSession, nil
}

func (dtlsSession *DTLSSession) getErrorHandler(rv C.int, errno error) func() error {
	errcode := C.SSL_get_error(dtlsSession.ssl, rv)
	switch errcode {
	case C.SSL_ERROR_ZERO_RETURN:
		return func() error {
			dtlsSession.Close()
			return io.ErrUnexpectedEOF
		}
	case C.SSL_ERROR_WANT_READ:
		go dtlsSession.flushOutputBuffer()
		if dtlsSession.want_read_future != nil {
			want_read_future := dtlsSession.want_read_future
			return func() error {
				_, err := want_read_future.Get()
				return err
			}
		}
		dtlsSession.want_read_future = NewFuture()
		want_read_future := dtlsSession.want_read_future
		return func() (err error) {
			defer func() {
				dtlsSession.mutex.Lock()
				dtlsSession.want_read_future = nil
				dtlsSession.mutex.Unlock()
				want_read_future.Set(nil, err)
			}()
			return tryAgain
		}
	case C.SSL_ERROR_WANT_WRITE:
		return func() error {
			err := dtlsSession.flushOutputBuffer()
			if err != nil {
				return err
			}
			return tryAgain
		}
	case C.SSL_ERROR_SYSCALL:
		var err error
		if C.ERR_peek_error() == 0 {
			switch rv {
			case 0:
				err = errors.New("protocol-violating EOF")
			case -1:
				err = errno
			default:
				err = errorFromErrorQueue()
			}
		} else {
			err = errorFromErrorQueue()
		}
		return func() error { return err }
	default:
		err := errorFromErrorQueue()
		return func() error { return err }
	}
}

func (dtlsSession *DTLSSession) handleError(errcb func() error) error {
	if errcb != nil {
		return errcb()
	}
	return nil
}

func (dtlsSession *DTLSSession) HandleData(data []byte) {
	// Here should Write to OpenSSL
	n, err := dtlsSession.intoOSSL.ReadFromOnce(data)
	if log.OnError(err, "could not write to OpenSSL (success of write %d byte)", n) {
		return
	}
	return
}

func (dtlsSession *DTLSSession) handshake() func() error {
	dtlsSession.mutex.Lock()
	defer dtlsSession.mutex.Unlock()
	if dtlsSession.isShutdown {
		return func() error { return io.ErrUnexpectedEOF }
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rc, errno := C.SSL_do_handshake(dtlsSession.ssl)
	if rc != 1 {
		return dtlsSession.getErrorHandler(rc, errno)
	}

	return nil
}

func (dtlsSession *DTLSSession) Handshake() (err error) {
	i, ts, err := 0, time.Now(), tryAgain
	for ; err == tryAgain; i++ {
		time.Sleep(20 * time.Millisecond)
		if i > 2000 {
			return
		}
		err = dtlsSession.handleError(dtlsSession.handshake())
	}
	log.Infof("handshake done after %d iterations (%v)", i, time.Now().Sub(ts))

	rc := 0
	//for rc != 1 {
	rc = int(C.SSL_is_init_finished(dtlsSession.ssl))
	//}*/

	log.Infof("SSL_is_init_finished is %d", int(rc))

	log.Infof("Handshake done, flushOutputBuffer()")
	dtlsSession.flushOutputBuffer()
	log.Infof("flushOutputBuffer() done")

	return
}

func (dtlsSession *DTLSSession) accept() func() error {
	dtlsSession.mutex.Lock()
	defer dtlsSession.mutex.Unlock()

	if dtlsSession.isShutdown {
		return func() error { return io.ErrUnexpectedEOF }
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rc, errno := C.SSL_accept(dtlsSession.ssl)
	if rc != 1 {
		return dtlsSession.getErrorHandler(rc, errno)
	}

	return nil
}

func (dtlsSession *DTLSSession) Accept() (err error) {
	err = tryAgain
	for err == tryAgain {
		err = dtlsSession.handleError(dtlsSession.accept())
	}

	rc := int(C.SSL_is_init_finished(dtlsSession.ssl))
	if rc != 1 {
		err = errors.New("DTLS connection is not finished and is not working")
		return
	}

	dtlsSession.flushOutputBuffer()
	log.Infof("Server Accept() done")

	return
}

func (dtlsSession *DTLSSession) GetSrtpKeys() (srtpKeys *SrtpKeys, err error) {
	srtpKeys, err = dtlsSession.ExportKeyingMaterialSrtp()

	return
}

func (dtlsSession *DTLSSession) flushOutputBuffer() error {
	_, err := dtlsSession.fromOSSL.WriteTo(dtlsSession.ch, dtlsSession.rAddr)
	return err
}
