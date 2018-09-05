package dtls

// #include "shim.h"
import "C"

import (
	"errors"
	"os"
	"runtime"
	"unsafe"
)

type Ctx struct {
	ctx       *C.SSL_CTX
	cert      *Certificate
	chain     []*Certificate
	key       PrivateKey
	verify_cb VerifyCallback
	sni_cb    TLSExtServernameCallback
}

type VerifyCallback func(preverify_ok bool, store *CertificateStoreCtx) bool
type CertificateStoreCtx struct {
	ctx     *C.X509_STORE_CTX
	ssl_ctx *Ctx
}
type TLSExtServernameCallback func(ssl *SSL) SSLTLSExtErr

type VerifyOptions int

const (
	VerifyNone             VerifyOptions = C.SSL_VERIFY_NONE
	VerifyPeer             VerifyOptions = C.SSL_VERIFY_PEER
	VerifyFailIfNoPeerCert VerifyOptions = C.SSL_VERIFY_FAIL_IF_NO_PEER_CERT
	VerifyClientOnce       VerifyOptions = C.SSL_VERIFY_CLIENT_ONCE
)

//export go_ssl_ctx_verify_cb_thunk
func go_ssl_ctx_verify_cb_thunk(p unsafe.Pointer, preverify_ok C.int, ctx *C.X509_STORE_CTX) (ok C.int) {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: verify callback panic'd %v", err)
			os.Exit(1)
		}
	}()

	ok = 0
	verify_cb := (*Ctx)(p).verify_cb
	// set up defaults just in case verify_cb is nil
	if verify_cb != nil {
		store := &CertificateStoreCtx{ctx: ctx}
		if verify_cb(preverify_ok == 1, store) {
			ok = 1
		}
	}

	return ok
}

type EllipticCurve int

const (
	Prime256v1 EllipticCurve = C.NID_X9_62_prime256v1
	Secp384r1  EllipticCurve = C.NID_secp384r1
)

func (c *Ctx) SetEllipticCurve(curve EllipticCurve) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	k := C.EC_KEY_new_by_curve_name(C.int(curve))
	if k == nil {
		return errors.New("Unknown curve")
	}
	defer C.EC_KEY_free(k)

	if int(C.X_SSL_CTX_set_tmp_ecdh(c.ctx, k)) != 1 {
		return errorFromErrorQueue()
	}

	return nil
}

type SSLVersion int

const (
	DTLSv1 SSLVersion = 0x01
)

var (
	ssl_ctx_idx = C.X_SSL_CTX_new_index()
)

func (c *Ctx) UseCertificate(cert *Certificate) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c.cert = cert
	if int(C.SSL_CTX_use_certificate(c.ctx, cert.x)) != 1 {
		return errorFromErrorQueue()
	}
	return nil
}

func (c *Ctx) AddChainCertificate(cert *Certificate) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c.chain = append(c.chain, cert)
	if int(C.X_SSL_CTX_add_extra_chain_cert(c.ctx, cert.x)) != 1 {
		return errorFromErrorQueue()
	}
	// OpenSSL takes ownership via SSL_CTX_add_extra_chain_cert
	runtime.SetFinalizer(cert, nil)
	return nil
}

func (c *Ctx) UsePrivateKey(key PrivateKey) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	c.key = key
	if int(C.SSL_CTX_use_PrivateKey(c.ctx, key.evpPKey())) != 1 {
		return errorFromErrorQueue()
	}
	return nil
}

//export get_ssl_ctx_idx
func get_ssl_ctx_idx() C.int {
	return ssl_ctx_idx
}

func newCtx(method *C.SSL_METHOD) (*Ctx, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx := C.SSL_CTX_new(method)
	if ctx == nil {
		return nil, errorFromErrorQueue()
	}
	c := &Ctx{ctx: ctx}
	C.SSL_CTX_set_ex_data(ctx, get_ssl_ctx_idx(), unsafe.Pointer(c))
	runtime.SetFinalizer(c, func(c *Ctx) {
		C.SSL_CTX_free(c.ctx)
	})
	return c, nil
}

func NewCtxWithVersion(version SSLVersion) (*Ctx, error) {
	var method *C.SSL_METHOD
	switch version {
	case DTLSv1:
		method = C.X_DTLS_method()
	}
	if method == nil {
		return nil, errors.New("unknown ssl/tls version")
	}
	return newCtx(method)
}

func (c *Ctx) SetVerify(options VerifyOptions, verify_cb VerifyCallback) {
	c.verify_cb = verify_cb
	if verify_cb != nil {
		C.SSL_CTX_set_verify(c.ctx, C.int(options), (*[0]byte)(C.X_SSL_CTX_verify_cb))
	} else {
		C.SSL_CTX_set_verify(c.ctx, C.int(options), nil)
	}
}

func (c *Ctx) SetTLSExtUseSrtp(profiles string) {
	p := C.CString(profiles)
	defer C.free(unsafe.Pointer(p))

	C.X_SSL_CTX_set_tlsext_use_srtp(c.ctx, p)
}

func (c *Ctx) SetCipherList(list string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	clist := C.CString(list)
	defer C.free(unsafe.Pointer(clist))
	if int(C.SSL_CTX_set_cipher_list(c.ctx, clist)) == 0 {
		return errorFromErrorQueue()
	}
	return nil
}

type Options int

const (
	OpSingleECDHUse Options = C.SSL_OP_SINGLE_ECDH_USE
	OpNoQueryMtu    Options = C.SSL_OP_NO_QUERY_MTU
)

func (c *Ctx) SetOptions(options Options) {
	C.X_SSL_CTX_set_options(c.ctx, C.long(options))
}

type Modes int

const (
	ReleaseBuffers Modes = C.SSL_MODE_RELEASE_BUFFERS
)

// SetMode sets context modes. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_mode.html
func (c *Ctx) SetMode(modes Modes) Modes {
	return Modes(C.X_SSL_CTX_set_mode(c.ctx, C.long(modes)))
}

// GetMode returns context modes. See
// http://www.openssl.org/docs/ssl/SSL_CTX_set_mode.html
func (c *Ctx) GetMode() Modes {
	return Modes(C.X_SSL_CTX_get_mode(c.ctx))
}
