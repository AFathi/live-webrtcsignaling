package srtp

//#include "shim.h"
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var errorString []string = []string{
	"srtp_err_status_ok",
	"srtp_err_status_fail",
	"srtp_err_status_bad_param",
	"srtp_err_status_alloc_fail",
	"srtp_err_status_dealloc_fail",
	"srtp_err_status_init_fail",
	"srtp_err_status_terminus",
	"srtp_err_status_auth_fail",
	"srtp_err_status_cipher_fail",
	"srtp_err_status_replay_fail",
	"srtp_err_status_replay_old",
	"srtp_err_status_algo_fail",
	"srtp_err_status_no_such_op",
	"srtp_err_status_no_ctx",
	"srtp_err_status_cant_check",
	"srtp_err_status_key_expired",
	"srtp_err_status_socket_err",
	"srtp_err_status_signal_err",
	"srtp_err_status_nonce_bad",
	"srtp_err_status_read_fail",
	"srtp_err_status_write_fail",
	"srtp_err_status_parse_err",
	"srtp_err_status_encode_err",
	"srtp_err_status_semaphore_err",
	"srtp_err_status_pfkey_err",
}

func init() {
	logString("[ SRTP ] Initialization")
	if rc := C.X_SRTP_shim_init(); rc != 0 {
		panic(fmt.Errorf("c_shimInit() failed with return code %d", rc))
	} else {
		logString("[ SRTP ] library libSRTP is initialized correctly")
	}
}

type SrtpSession struct {
	LocalPolicy  *C.srtp_policy_t
	RemotePolicy *C.srtp_policy_t
	SrtpOut      *Srtp
	SrtpIn       *Srtp
}

func nonCopyGoBytes(ptr uintptr, length int) []byte {
	var slice []byte
	header := (*reflect.SliceHeader)(unsafe.Pointer(&slice))
	header.Cap = length
	header.Len = length
	header.Data = ptr
	return slice
}

func nonCopyCString(data *C.char, size C.int) []byte {
	return nonCopyGoBytes(uintptr(unsafe.Pointer(data)), int(size))
}

type Srtp struct {
	sync.RWMutex
	C C.srtp_t
}

var srtpGlobalMutex sync.RWMutex

/*func New() (srtp *Srtp, err error) {
	Csrtp := C.X_SRTP_new()
	if Csrtp == nil {
		err = errors.New("could not allocate a new Srtp/srtp_t")
		return
	}
	srtp = &Srtp{
		C: Csrtp,
	}

	runtime.SetFinalizer(srtp, func(srtp *Srtp) {
		C.free(unsafe.Pointer(srtp.C))
	})

	return
}*/

func Create(localSrtpKey []byte, remoteSrtpKey []byte) (srtpSession *SrtpSession, err error) {
	srtpGlobalMutex.Lock()
	defer srtpGlobalMutex.Unlock()

	logString("[ SRTP ] localSrtpKey is %#v, remoteSrtpKey is %#v", localSrtpKey, remoteSrtpKey)
	localPolicy := C.X_SRTP_set_local_policy((*C.uchar)(unsafe.Pointer(&localSrtpKey[0])))
	remotePolicy := C.X_SRTP_set_remote_policy((*C.uchar)(unsafe.Pointer(&remoteSrtpKey[0])))

	logString("[ SRTP ] remotePolicy == %#v", remotePolicy)
	var ret int
	Csrtp := C.X_srtp_create(remotePolicy, (*C.srtp_err_status_t)(unsafe.Pointer(&ret)))
	if ret != C.srtp_err_status_ok {
		err = errors.New(fmt.Sprintf("error creating inbound SRTP session: %s", errorString[int(ret)]))
		return
	}
	srtpIn := &Srtp{
		C: Csrtp,
	}
	logString("[ SRTP ] localPolicy == %#v", localPolicy)
	Csrtp = C.X_srtp_create(localPolicy, (*C.srtp_err_status_t)(unsafe.Pointer(&ret)))
	if ret != C.srtp_err_status_ok {
		err = errors.New(fmt.Sprintf("error creating outbound SRTP session: %s", errorString[int(ret)]))
		return
	}
	srtpOut := &Srtp{
		C: Csrtp,
	}

	srtpSession = &SrtpSession{
		LocalPolicy:  localPolicy,
		RemotePolicy: remotePolicy,
		SrtpIn:       srtpIn,
		SrtpOut:      srtpOut,
	}

	runtime.SetFinalizer(srtpSession, func(s *SrtpSession) {
		C.srtp_dealloc(s.SrtpIn.C)
		C.srtp_dealloc(s.SrtpOut.C)
		C.free(unsafe.Pointer(s.LocalPolicy))
		C.free(unsafe.Pointer(s.RemotePolicy))
	})

	logString("[ SRTP ] srtpSession is %#v", srtpSession)

	return
}

func UnprotectRTP(ctx *Srtp, data []byte) (int, error) {
	/*ctx.Lock()
	defer ctx.Unlock()*/

	bufLength := len(data)
	//logString("[ SRTP ] Unprotect packet %#b, length %d", data, len(data))

	res := C.srtp_unprotect(ctx.C, unsafe.Pointer(&data[0]), (*C.int)(unsafe.Pointer(&bufLength)))
	if res != C.srtp_err_status_ok {
		return bufLength, errors.New(fmt.Sprintf("%s", errorString[int(res)]))
	}
	//logString("[ SRTP ] crypted packet size is %d, decyrpted packet size is %d", len(data), bufLength)
	return bufLength, nil
}

/*
  This function unprotect SRTCP => RTCP packet

	SRTCP https://tools.ietf.org/html/rfc3711#section-3.4

	 0                   1                   2                   3
	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+<+
	|V=2|P|    RC   |   PT=SR or RR   |             length          | |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	|                         SSRC of sender                        | |
	+>+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+ |
	| ~                          sender info                          ~ |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| ~                         report block 1                        ~ |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| ~                         report block 2                        ~ |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| ~                              ...                              ~ |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| |V=2|P|    SC   |  PT=SDES=202  |             length            | |
	| +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+ |
	| |                          SSRC/CSRC_1                          | |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| ~                           SDES items                          ~ |
	| +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+ |
	| ~                              ...                              ~ |
	+>+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+ |
	| |E|                         SRTCP index                         | |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+<+
	| ~                     SRTCP MKI (OPTIONAL)                      ~ |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	| :                     authentication tag                        : |
	| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |
	|                                                                   |
	+-- Encrypted Portion                    Authenticated Portion -----+

*/

func Unprotect(ctx *Srtp, input IPacketUDP) (IPacketUDP, error) {
	// assuming data[0] >= 128 && data[0] <= 191
	data := input.GetData()
	pt := int(data[1] & 0x7F) // 0111 1111
	switch {
	case pt < 64 || pt > 95:
		rtpPacket, err := NewPacketSRTP(input).Unprotect(ctx)
		if err != nil {
			return nil, err
		}
		return IPacketUDP(rtpPacket), nil
	case pt >= 64 && pt <= 95:
		rtcpPacket, err := NewPacketSRTCP(input).Unprotect(ctx)
		if err != nil {
			return nil, err
		}
		return IPacketUDP(rtcpPacket), nil
	default:
		fmt.Printf("Unprotect error: neither SRTP nor SRTCP packet\n")
		return nil, errors.New("neither SRTP nor SRTCP")
	}
}

func Protect(ctx *Srtp, data []byte) (newSize int, err error) {
	/*ctx.Lock()
	defer ctx.Unlock()*/
	lenData := len(data)
	bufLength := lenData
	//logString("[ SRTP ] Protect packet %#v, length %d", data, len(data))
	res := C.srtp_protect(ctx.C, unsafe.Pointer(&data[0]), (*C.int)(unsafe.Pointer(&bufLength)))
	if res != C.srtp_err_status_ok {
		newSize = bufLength
		err = errors.New(fmt.Sprintf("could not encode SRTP packet: %s", errorString[int(res)]))
		return
	}
	newSize = bufLength
	//logString("[ SRTP ] unciphered packet size is %d, ciphered packet size is %d", bufLength, len(data))

	return
}

func ProtectRtcp(ctx *Srtp, data []byte) (newSize int, err error) {
	/*ctx.Lock()
	defer ctx.Unlock()*/

	bufLength := len(data)
	res := C.srtp_protect_rtcp(ctx.C, unsafe.Pointer(&data[0]), (*C.int)(unsafe.Pointer(&bufLength)))
	if res != C.srtp_err_status_ok {
		newSize = bufLength
		err = errors.New(fmt.Sprintf("could not encore SRTP RTCP packet: %s", errorString[int(res)]))
		return
	}
	newSize = bufLength

	return
}

func UnprotectRTCP(ctx *Srtp, data []byte) (int, error) {
	/*ctx.Lock()
	defer ctx.Unlock()*/

	bufLength := len(data)
	//logString("[ SRTP ] Unprotect packet %#v, length %d", data, len(data))
	res := C.srtp_unprotect_rtcp(ctx.C, unsafe.Pointer(&data[0]), (*C.int)(unsafe.Pointer(&bufLength)))
	if res != C.srtp_err_status_ok {
		return bufLength, errors.New(fmt.Sprintf("%s", errorString[int(res)]))
	}
	//logString("[ SRTP ] crypted packet size is %d, decyrpted packet size is %d", len(data), bufLength)
	return bufLength, nil
}
