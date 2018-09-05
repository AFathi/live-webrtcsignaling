#include <string.h>

#include <openssl/conf.h>

#include <openssl/bio.h>
#include <openssl/crypto.h>
#include <openssl/engine.h>
#include <openssl/err.h>
#include <openssl/evp.h>
#include <openssl/ssl.h>

#include <pthread.h>

#include "_cgo_export.h"

pthread_mutex_t* opensslLocks;

static int go_sinkWritePuts(BIO *b, const char *str) {
	return go_sinkWrite(b, (char*)str, (int)strlen(str));
}

#if OPENSSL_VERSION_NUMBER >= 0x1010000fL

void X_BIO_set_data(BIO* bio, void* data) {
	BIO_set_data(bio, data);
}

void* X_BIO_get_data(BIO* bio) {
	return BIO_get_data(bio);
}

EVP_MD_CTX* X_EVP_MD_CTX_new() {
	return EVP_MD_CTX_create();
}

void X_EVP_MD_CTX_free(EVP_MD_CTX* ctx) {
	EVP_MD_CTX_destroy(ctx);
}

static int bioCreate(BIO *b) {
	BIO_set_shutdown(b, 1);
	BIO_set_init(b, 1);
	BIO_set_data(b, NULL);
	BIO_set_flags(b, 0);

	return 1;
}

static int bioFree(BIO *b) {
	return 1;
}

static BIO_METHOD *writeBioMethod = NULL;
static BIO_METHOD *readBioMethod = NULL;

BIO_METHOD* BIO_s_readBio() { return readBioMethod; }
BIO_METHOD* BIO_s_writeBio() { return writeBioMethod; }

int bioInitMethods() {
	writeBioMethod = BIO_meth_new(BIO_TYPE_SOURCE_SINK, "tribeMCU Write BIO");
	if (! writeBioMethod) {
		return 1;
	}
	if (BIO_meth_set_write(writeBioMethod, (int (*)(BIO *, const char *, int))go_sinkWrite) != 1) {
		return 2;
	}
	if (BIO_meth_set_puts(writeBioMethod, go_sinkWritePuts) != 1) {
    return 3;
  }
	if (BIO_meth_set_ctrl(writeBioMethod, go_sinkWriteCtrl) != 1) {
		return 4;
	}
	if (BIO_meth_set_create(writeBioMethod, bioCreate) != 1) {
		return 5;
	}
	if (BIO_meth_set_destroy(writeBioMethod, bioFree) != 1) {
		return 6;
	}

	readBioMethod = BIO_meth_new(BIO_TYPE_SOURCE_SINK, "tribeMCU Read BIO");
	if (! readBioMethod) {
	  return 7;
	}
  if (BIO_meth_set_read(readBioMethod, go_sinkRead) != 1) {
    return 8;
  }
  if (BIO_meth_set_ctrl(readBioMethod, go_sinkReadCtrl) != 1) {
    return 9;
  }
  if (BIO_meth_set_create(readBioMethod, bioCreate) != 1) {
    return 10;
  }
  if (BIO_meth_set_destroy(readBioMethod, bioFree) != 1) {
    return 11;
  }

	return 0;
}

#endif

#if OPENSSL_VERSION_NUMBER < 0x1010000fL

static int bioCreate(BIO *b) {
	b->shutdown = 1;
	b->init = 1;
	b->num = -1;
	b->ptr = NULL;
	b->flags = 0;
	return 1;
}

static int bioFree(BIO *b) {
	return 1;
}

static BIO_METHOD writeBioMethod = {
	BIO_TYPE_SOURCE_SINK,
	"tribeMCU Write BIO",
	(int (*)(BIO *, const char *, int))go_sinkWrite,
	NULL,
	go_sinkWritePuts,
	NULL,
	go_sinkWriteCtrl,
	bioCreate,
	bioFree,
	NULL};

static BIO_METHOD* BIO_s_writeBio() { return &writeBioMethod; }

static BIO_METHOD readBioMethod = {
	BIO_TYPE_SOURCE_SINK,
	"tribeMCU Read BIO",
	NULL,
	go_sinkRead,
	NULL,
	NULL,
	go_sinkReadCtrl,
	bioCreate,
	bioFree,
	NULL};

static BIO_METHOD* BIO_s_readBio() { return &readBioMethod; }

int bioInitMethods() {
	/* statically initialized above */
	return 0;
}

void X_BIO_set_data(BIO* bio, void* data) {
	bio->ptr = data;
}

void* X_BIO_get_data(BIO* bio) {
	return bio->ptr;
}

EVP_MD_CTX* X_EVP_MD_CTX_new() {
	return EVP_MD_CTX_create();
}

void X_EVP_MD_CTX_free(EVP_MD_CTX* ctx) {
	EVP_MD_CTX_destroy(ctx);
}

#endif

EVP_PKEY *X_EVP_PKEY_new(void) {
	return EVP_PKEY_new();
}

struct rsa_st *X_EVP_PKEY_get1_RSA(EVP_PKEY *pkey) {
        return EVP_PKEY_get1_RSA(pkey);
}

int X_EVP_PKEY_size(EVP_PKEY *pkey) {
	return EVP_PKEY_size(pkey);
}

int X_EVP_PKEY_set1_RSA(EVP_PKEY *pkey, struct rsa_st *key) {
	return EVP_PKEY_set1_RSA(pkey, key);
}

void X_EVP_PKEY_free(EVP_PKEY *pkey) {
        EVP_PKEY_free(pkey);
}

int X_EVP_SignInit(EVP_MD_CTX *ctx, const EVP_MD *type) {
	return EVP_SignInit(ctx, type);
}

int X_EVP_SignUpdate(EVP_MD_CTX *ctx, const void *d, unsigned int cnt) {
	return EVP_SignUpdate(ctx, d, cnt);
}

int X_EVP_SignFinal(EVP_MD_CTX *ctx, unsigned char *md, unsigned int *s, EVP_PKEY *pkey) {
	return EVP_SignFinal(ctx, md, s, pkey);
}

int X_EVP_VerifyInit(EVP_MD_CTX *ctx, const EVP_MD *type) {
	return EVP_VerifyInit(ctx, type);
}

int X_EVP_VerifyUpdate(EVP_MD_CTX *ctx, const void *d,
		unsigned int cnt) {
	return EVP_VerifyUpdate(ctx, d, cnt);
}

int X_EVP_VerifyFinal(EVP_MD_CTX *ctx, const unsigned char *sigbuf, unsigned int siglen, EVP_PKEY *pkey) {
	return EVP_VerifyFinal(ctx, sigbuf, siglen, pkey);
}

const EVP_MD *X_EVP_sha256() {
        return EVP_sha256();
}

const EVP_MD *X_EVP_sha1() {
	return EVP_sha1();
}

const EVP_MD *X_EVP_sha512() {
	return EVP_sha512();
}

const SSL_METHOD *X_DTLS_method() {
#if OPENSSL_VERSION_NUMBER >= 0x1010000fL
  return DTLS_method();
#else
  return DTLSv1_method();
#endif
}

int X_SSL_CTX_new_index() {
        return SSL_CTX_get_ex_new_index(0, NULL, NULL, NULL, NULL);
}

long X_SSL_CTX_set_options(SSL_CTX* ctx, long options) {
        return SSL_CTX_set_options(ctx, options);
}

long X_SSL_CTX_add_extra_chain_cert(SSL_CTX* ctx, X509 *cert) {
        return SSL_CTX_add_extra_chain_cert(ctx, cert);
}

long X_SSL_CTX_set_tmp_ecdh(SSL_CTX* ctx, EC_KEY *key) {
        return SSL_CTX_set_tmp_ecdh(ctx, key);
}

int X_SSL_CTX_set_tlsext_use_srtp(SSL_CTX * ctx, const char *profiles) {
        return SSL_CTX_set_tlsext_use_srtp(ctx, profiles);
}

int X_SSL_CTX_verify_cb(int preverify_ok, X509_STORE_CTX* store) {
  printf("X_SSL_CTX_verify_cb preverify is %d\n", preverify_ok);
        SSL* ssl = (SSL *)X509_STORE_CTX_get_ex_data(store,
                        SSL_get_ex_data_X509_STORE_CTX_idx());
        SSL_CTX* ssl_ctx = SSL_get_SSL_CTX(ssl);
        void* p = SSL_CTX_get_ex_data(ssl_ctx, get_ssl_ctx_idx());
        // get the pointer to the go Ctx object and pass it back into the thunk
        return go_ssl_ctx_verify_cb_thunk(p, preverify_ok, store);
}

long X_SSL_CTX_set_mode(SSL_CTX* ctx, long modes) {
	return SSL_CTX_set_mode(ctx, modes);
}

long X_SSL_CTX_get_mode(SSL_CTX* ctx) {
	return SSL_CTX_get_mode(ctx);
}

BIO *X_BIO_new_write_bio() {
	return BIO_new(BIO_s_writeBio());
}

BIO *X_BIO_new_read_bio() {
	return BIO_new(BIO_s_readBio());
}

void X_BIO_set_flags(BIO *b, int flags) {
	return BIO_set_flags(b, flags);
}

void X_BIO_clear_flags(BIO *b, int flags) {
        BIO_clear_flags(b, flags);
}

int X_BIO_read(BIO *b, void *buf, int len) {
	return BIO_read(b, buf, len);
}

int X_BIO_write(BIO *b, const void *buf, int len) {
	return BIO_write(b, buf, len);
}

int X_SSL_verify_cb(int preverify_ok, X509_STORE_CTX* store) {
	SSL* ssl = (SSL *)X509_STORE_CTX_get_ex_data(store,
			SSL_get_ex_data_X509_STORE_CTX_idx());
	void* p = SSL_get_ex_data(ssl, get_ssl_idx());
	// get the pointer to the go Ctx object and pass it back into the thunk
	return go_ssl_verify_cb_thunk(p, preverify_ok, store);
}

void X_SSL_info_cb(SSL *ssl, int where, int ret) {
  void *p = SSL_get_ex_data(ssl, get_ssl_idx());
  return go_ssl_info_cb_thunk(p, where, ret);
}

int X_SSL_new_index() {
	return SSL_get_ex_new_index(0, NULL, NULL, NULL, NULL);
}

void X_SSL_set_mtu(SSL *ssl, int mtu) {
  SSL_set_mtu(ssl, mtu);
}

// Mutex locking management functions
int initLocks() {
  int rc = 0;
  int nlock;
  int i;
  int locksNeeded = CRYPTO_num_locks();

  opensslLocks = (pthread_mutex_t*)malloc(sizeof(pthread_mutex_t) * locksNeeded);
  if (! opensslLocks) {
    return ENOMEM;
  }
  for (nlock = 0; nlock < locksNeeded; ++nlock) {
    rc = pthread_mutex_init(&opensslLocks[nlock], NULL);
    if (rc != 0) {
      break;
    }
  }

  if (rc != 0) {
    for (i = nlock - 1; i >= 0; --i) {
      pthread_mutex_destroy(&opensslLocks[i]);
    }
    free(opensslLocks);
    opensslLocks = NULL;
  }

  return rc;
}

void threadLockingCallback(int mode, int n, const char *file, int line) {
  if (mode & CRYPTO_LOCK) {
    pthread_mutex_lock(&opensslLocks[n]);
  } else {
    pthread_mutex_unlock(&opensslLocks[n]);
  }
}

// Finally initialization of shim
int shimInit() {
  int rc = 0;

  //OPENSSL_config(NULL);
  ENGINE_load_builtin_engines();
  SSL_library_init();
  SSL_load_error_strings();
  OpenSSL_add_all_algorithms();
  //
  // Set up OPENSSL thread safety callbacks.  We only set the locking
  // callback because the default id callback implementation is good
  // enough for us.
  rc = initLocks();
  if (rc != 0) {
    return rc;
  }
  CRYPTO_set_locking_callback(threadLockingCallback);

  rc = bioInitMethods();
  if (rc != 0) {
    return rc;
  }

  return 0;
}
