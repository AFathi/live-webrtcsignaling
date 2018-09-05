package dtls

// #include "shim.h"
import "C"

import (
	"errors"
	"os"
	"unsafe"
)

type SSLTLSExtErr int

const (
	SSLTLSExtErrOK           SSLTLSExtErr = C.SSL_TLSEXT_ERR_OK
	SSLTLSExtErrAlertWarning SSLTLSExtErr = C.SSL_TLSEXT_ERR_ALERT_WARNING
	SSLTLSEXTErrAlertFatal   SSLTLSExtErr = C.SSL_TLSEXT_ERR_ALERT_FATAL
	SSLTLSEXTErrNoAck        SSLTLSExtErr = C.SSL_TLSEXT_ERR_NOACK
)

const (
	SSLSrtpMasterKeyLength  int = 16
	SSLSrtpMasterSaltLength int = 14
	SSLSrtpMasterLength     int = SSLSrtpMasterKeyLength + SSLSrtpMasterSaltLength
)

var (
	ssl_idx = C.X_SSL_new_index()
)

//export get_ssl_idx
func get_ssl_idx() C.int {
	return ssl_idx
}

type SSL struct {
	ssl       *C.SSL
	verify_cb VerifyCallback
	info_cb   InfoCallback
}

//export go_ssl_verify_cb_thunk
func go_ssl_verify_cb_thunk(p unsafe.Pointer, preverify_ok C.int, ctx *C.X509_STORE_CTX) (ok C.int) {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: verify callback panic'd: %v", err)
			os.Exit(1)
		}
	}()

	ok = 0
	verify_cb := (*SSL)(p).verify_cb
	// set up defaults just in case verify_cb is nil
	if verify_cb != nil {
		store := &CertificateStoreCtx{ctx: ctx}
		if verify_cb(preverify_ok == 1, store) {
			ok = 1
		} else {
			ok = 0
		}
	}

	return ok
}

// Wrapper around SSL_get_servername. Returns server name according to rfc6066
// http://tools.ietf.org/html/rfc6066.
func (s *SSL) GetServername() string {
	return C.GoString(C.SSL_get_servername(s.ssl, C.TLSEXT_NAMETYPE_host_name))
}

// SetVerify controls peer verification settings. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) SetVerify(options VerifyOptions, verify_cb VerifyCallback) {
	s.verify_cb = verify_cb
	if verify_cb != nil {
		C.SSL_set_verify(s.ssl, C.int(options), (*[0]byte)(C.X_SSL_verify_cb))
	} else {
		C.SSL_set_verify(s.ssl, C.int(options), nil)
	}
}

// SetVerifyMode controls peer verification setting. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) SetVerifyMode(options VerifyOptions) {
	s.SetVerify(options, s.verify_cb)
}

// SetVerifyCallback controls peer verification setting. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) SetVerifyCallback(verify_cb VerifyCallback) {
	s.SetVerify(s.VerifyMode(), verify_cb)
}

// GetVerifyCallback returns callback function. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) GetVerifyCallback() VerifyCallback {
	return s.verify_cb
}

// VerifyMode returns peer verification setting. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) VerifyMode() VerifyOptions {
	return VerifyOptions(C.SSL_get_verify_mode(s.ssl))
}

// SetVerifyDepth controls how many certificates deep the certificate
// verification logic is willing to follow a certificate chain. See
// https://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) SetVerifyDepth(depth int) {
	C.SSL_set_verify_depth(s.ssl, C.int(depth))
}

// GetVerifyDepth controls how many certificates deep the certificate
// verification logic is willing to follow a certificate chain. See
// https://www.openssl.org/docs/ssl/SSL_CTX_set_verify.html
func (s *SSL) GetVerifyDepth() int {
	return int(C.SSL_get_verify_depth(s.ssl))
}

// SetSSLCtx changes context to new one. Useful for Server Name Indication (SNI)
// rfc6066 http://tools.ietf.org/html/rfc6066. See
// http://stackoverflow.com/questions/22373332/serving-multiple-domains-in-one-box-with-sni
func (s *SSL) SetSSLCtx(ctx *Ctx) {
	/*
	 * SSL_set_SSL_CTX() only changes certs as of 1.0.0d
	 * adjust other things we care about
	 */
	C.SSL_set_SSL_CTX(s.ssl, ctx.ctx)
}

type SrtpKeys struct {
	LocalKey   []byte
	RemoteKey  []byte
	LocalSalt  []byte
	RemoteSalt []byte
}

func (s *SSL) ExportKeyingMaterialSrtp() (srtpKeys *SrtpKeys, err error) {
	label := C.CString("EXTRACTOR-dtls_srtp")
	defer C.free(unsafe.Pointer(label))
	material := make([]byte, SSLSrtpMasterLength*2)
	if 1 != C.SSL_export_keying_material(s.ssl, (*C.uchar)(unsafe.Pointer(&material[0])), C.size_t(len(material)), label, C.size_t(19), nil, C.size_t(0), C.int(0)) {
		err = errors.New("Could not get keying material")
		return
	}

	log.Infof("material is %#v", material)

	srtpKeys = &SrtpKeys{
		LocalKey:   material[:SSLSrtpMasterKeyLength],
		RemoteKey:  material[SSLSrtpMasterKeyLength : SSLSrtpMasterKeyLength*2],
		LocalSalt:  material[SSLSrtpMasterKeyLength*2 : SSLSrtpMasterKeyLength*2+SSLSrtpMasterSaltLength],
		RemoteSalt: material[SSLSrtpMasterKeyLength*2+SSLSrtpMasterSaltLength : SSLSrtpMasterKeyLength*2+SSLSrtpMasterSaltLength*2],
	}

	return
}

//export sni_cb_thunk
func sni_cb_thunk(p unsafe.Pointer, con *C.SSL, ad unsafe.Pointer, arg unsafe.Pointer) C.int {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: verify callback sni panic'd: %v", err)
			os.Exit(1)
		}
	}()

	sni_cb := (*Ctx)(p).sni_cb

	s := &SSL{ssl: con}
	// This attaches a pointer to our SSL struct into the SNI callback.
	C.SSL_set_ex_data(s.ssl, get_ssl_idx(), unsafe.Pointer(s))

	// Note: this is ctx.sni_cb, not C.sni_cb
	return C.int(sni_cb(s))
}

type InfoCallback func(s *C.SSL, where int, ret int)

func (s *SSL) SetInfoCallback(infoCb InfoCallback) {
	s.info_cb = infoCb
	C.SSL_set_info_callback(s.ssl, (*[0]byte)(C.X_SSL_info_cb))
}

//export go_ssl_info_cb_thunk
func go_ssl_info_cb_thunk(p unsafe.Pointer, where C.int, ret C.int) {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: info callback panic'd: %v", err)
			os.Exit(1)
		}
	}()
	info_cb := (*SSL)(p).info_cb
	if info_cb != nil {
		info_cb((*SSL)(p).ssl, int(where), int(ret))
	}
	return
}
