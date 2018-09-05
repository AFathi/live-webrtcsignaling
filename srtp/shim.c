#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <inttypes.h>

#include <srtp2/srtp.h>

#include "_cgo_export.h"

int X_SRTP_shim_init() {
  fprintf(stderr, "[ SRTP ] shim init\n");
  if(srtp_init() != srtp_err_status_ok) {
    return -1;
  }

  return 0;
}

srtp_policy_t *X_SRTP_set_remote_policy(unsigned char *remote_srtp_key) {
  unsigned char *remote_srtp_key_copy;
  srtp_policy_t *remote_policy;

  remote_policy = (srtp_policy_t *)malloc(sizeof(srtp_policy_t));
  memset(remote_policy, 0x0, sizeof(srtp_policy_t));
  srtp_crypto_policy_set_rtp_default(&(remote_policy->rtp));
  srtp_crypto_policy_set_rtcp_default(&(remote_policy->rtcp));
  remote_policy->ssrc.type = ssrc_any_inbound;
  remote_policy->key = (unsigned char *)malloc(30);
  bcopy(remote_srtp_key,  remote_policy->key, 30);
  remote_policy->window_size = 128;
  remote_policy->allow_repeat_tx = 0;
  remote_policy->next = NULL;
  int i = 0;
  fprintf(stderr, "[ SRTP ] remote_policy->key is ");
  for (i = 0; i < 30; i++) {
    fprintf(stderr, "0x%.2x ",  remote_policy->key[i]);
  }
  fprintf(stderr, "\n");

  return remote_policy;
}

srtp_policy_t *X_SRTP_set_local_policy(unsigned char *local_srtp_key) {
  unsigned char *local_srtp_key_copy;
  srtp_policy_t *local_policy;

  local_policy = (srtp_policy_t *)malloc(sizeof(srtp_policy_t));
  memset(local_policy, 0x0, sizeof(srtp_policy_t));
  srtp_crypto_policy_set_rtp_default(&(local_policy->rtp));
  srtp_crypto_policy_set_rtcp_default(&(local_policy->rtcp));
  local_policy->ssrc.type = ssrc_any_outbound;
  local_policy->key = (unsigned char *)malloc(30);
  bcopy(local_srtp_key, local_policy->key, 30);
  local_policy->window_size = 128;
  local_policy->allow_repeat_tx = 0;
  local_policy->next = NULL;

  int i = 0;
  fprintf(stderr, "[ SRTP ] local_policy->key is ");
  for (i = 0; i < 30; i++) {
    fprintf(stderr, "0x%.2x ", local_policy->key[i]);
  }
  fprintf(stderr, "\n");

  return local_policy;
}

void X_SRTP_policy_free(srtp_policy_t *srtp_policy) {
  free(srtp_policy);

  return;
}

srtp_t X_srtp_create(const srtp_policy_t *policy, srtp_err_status_t *ret) {
  srtp_t session;
  fprintf(stderr, "[ SRTP ] session is %p, policy is %p\n", session, policy);
  *ret = srtp_create(&session, policy);
  fprintf(stderr, "[ SRTP ] End of X_srtp_create with ret = %d\n", *ret);
  return session;
}
