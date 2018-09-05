package dtls

//#include "shim.h"
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

type Certificate struct {
	x      *C.X509
	Issuer *Certificate
	ref    interface{}
	pubKey PublicKey
}

type EVP_MD int

const (
	EVP_NULL      EVP_MD = iota
	EVP_MD5       EVP_MD = iota
	EVP_SHA       EVP_MD = iota
	EVP_SHA1      EVP_MD = iota
	EVP_DSS       EVP_MD = iota
	EVP_DSS1      EVP_MD = iota
	EVP_RIPEMD160 EVP_MD = iota
	EVP_SHA224    EVP_MD = iota
	EVP_SHA256    EVP_MD = iota
	EVP_SHA384    EVP_MD = iota
	EVP_SHA512    EVP_MD = iota
)

func getDigestFunction(digest EVP_MD) (md *C.EVP_MD) {
	switch digest {
	case EVP_SHA256:
		md = C.X_EVP_sha256()
	}
	return md
}

// LoadCertificateFromPEM loads an X509 certificate from a PEM-encoded block.
func LoadCertificateFromPEM(pem_block []byte) (*Certificate, error) {
	if len(pem_block) == 0 {
		return nil, errors.New("empty pem block")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	bio := C.BIO_new_mem_buf(unsafe.Pointer(&pem_block[0]),
		C.int(len(pem_block)))
	cert := C.PEM_read_bio_X509(bio, nil, nil, nil)
	C.BIO_free(bio)
	if cert == nil {
		return nil, errorFromErrorQueue()
	}
	x := &Certificate{x: cert}
	runtime.SetFinalizer(x, func(x *Certificate) {
		C.X509_free(x.x)
	})
	return x, nil
}

// X509Digest returns a digest of the DER representation of the public key
func (c *Certificate) Digest(digest EVP_MD) ([]byte, error) {
	var md *C.EVP_MD = getDigestFunction(digest)
	var fingerprint [C.EVP_MAX_MD_SIZE]byte
	var len C.uint
	if C.X509_digest(c.x, md, (*C.uchar)(unsafe.Pointer(&fingerprint[0])), &len) == 0 {
		return nil, errors.New("could not compute digest")
	}
	return fingerprint[:len], nil
}
