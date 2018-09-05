#include <openssl/rand.h>
#include <openssl/err.h>
#include <srtp2/srtp.h>

extern int X_SRTP_shim_init();
extern srtp_policy_t *X_SRTP_set_remote_policy(unsigned char *remote_srtp_key);
extern srtp_policy_t *X_SRTP_set_local_policy(unsigned char *local_srtp_key);
extern void X_SRTP_policy_free(srtp_policy_t *srtp_policy);
extern srtp_t X_srtp_create(const srtp_policy_t *policy, srtp_err_status_t *ret);
