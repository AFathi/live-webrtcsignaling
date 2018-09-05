#include <stdlib.h>
#include <string.h>

#include <openssl/bio.h>
#include <openssl/crypto.h>
#include <openssl/dh.h>
#include <openssl/err.h>
#include <openssl/evp.h>
#include <openssl/hmac.h>
#include <openssl/pem.h>
#include <openssl/ssl.h>
#include <openssl/x509v3.h>

extern int shimInit();
extern int bioInitMethods();
extern const EVP_MD *X_EVP_sha256();
extern EVP_MD_CTX* X_EVP_MD_CTX_new();
extern void X_EVP_MD_CTX_free(EVP_MD_CTX* ctx);
extern EVP_PKEY *X_EVP_PKEY_new(void);
extern struct rsa_st *X_EVP_PKEY_get1_RSA(EVP_PKEY *pkey);
extern int X_EVP_PKEY_set1_RSA(EVP_PKEY *pkey, struct rsa_st *key);
extern void X_EVP_PKEY_free(EVP_PKEY *pkey);
extern int X_EVP_PKEY_size(EVP_PKEY *pkey);
extern int X_EVP_SignInit(EVP_MD_CTX *ctx, const EVP_MD *type);
extern int X_EVP_SignUpdate(EVP_MD_CTX *ctx, const void *d, unsigned int cnt);
extern int X_EVP_SignFinal(EVP_MD_CTX *ctx, unsigned char *md, unsigned int *s, EVP_PKEY *pkey);
extern int X_EVP_VerifyInit(EVP_MD_CTX *ctx, const EVP_MD *type);
extern int X_EVP_VerifyUpdate(EVP_MD_CTX *ctx, const void *d, unsigned int cnt);
extern int X_EVP_VerifyFinal(EVP_MD_CTX *ctx, const unsigned char *sigbuf, unsigned int siglen, EVP_PKEY *pkey);
extern const SSL_METHOD *X_DTLS_method();
extern long X_SSL_CTX_add_extra_chain_cert(SSL_CTX* ctx, X509 *cert);
extern int X_SSL_CTX_new_index();
extern long X_SSL_CTX_set_options(SSL_CTX* ctx, long options);
extern long X_SSL_CTX_set_tmp_ecdh(SSL_CTX* ctx, EC_KEY *key);
extern int X_SSL_CTX_set_tlsext_use_srtp(SSL_CTX * ctx, const char *profiles);
extern int X_SSL_CTX_verify_cb(int preverify_ok, X509_STORE_CTX* store);
extern long X_SSL_CTX_set_mode(SSL_CTX* ctx, long modes);
extern long X_SSL_CTX_get_mode(SSL_CTX* ctx);
extern BIO *X_BIO_new_write_bio();
extern BIO *X_BIO_new_read_bio();
extern void X_BIO_set_data(BIO *bio, void* data);
extern void *X_BIO_get_data(BIO *bio);
extern void X_BIO_set_flags(BIO *bio, int flags);
extern void X_BIO_clear_flags(BIO *b, int flags);
extern int X_BIO_read(BIO *b, void *buf, int len);
extern int X_BIO_write(BIO *b, const void *buf, int len);
extern const EVP_MD *X_EVP_sha1();
extern const EVP_MD *X_EVP_sha512();
extern int X_SSL_new_index();
extern int X_SSL_verify_cb(int preverify_ok, X509_STORE_CTX* store);
extern void X_SSL_info_cb(SSL *s, int where, int ret);
extern void X_SSL_set_mtu(SSL *ssl, int mtu);
