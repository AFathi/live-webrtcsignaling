package dtls

// #include "shim.h"
import "C"

import (
	"errors"
	"io"
	"net"
	"reflect"
	"unsafe"

	"github.com/heytribe/live-webrtcsignaling/my"
	"github.com/heytribe/live-webrtcsignaling/packet"
)

const (
	mtuFallback   C.long = 548
	SSLRecordSize        = 1500
)

var (
	writeBioMapping        = newMapping()
	mtu             C.long = 1200
)

type writeBio struct {
	dataMutex       my.Mutex
	opMutex         my.Mutex
	buf             []byte
	release_buffers bool
}

func loadWritePtr(b *C.BIO) *writeBio {
	t := unsafe.Pointer(C.X_BIO_get_data(b))
	return (*writeBio)(writeBioMapping.Get(t))
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

func bioClearRetryFlags(b *C.BIO) {
	C.X_BIO_clear_flags(b, C.BIO_FLAGS_RWS|C.BIO_FLAGS_SHOULD_RETRY)
}

//export go_sinkWrite
func go_sinkWrite(b *C.BIO, data *C.char, size C.int) (rc C.int) {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: go_sinkWrite panic'd; %v", err)
			rc = -1
		}
	}()

	ptr := loadWritePtr(b)
	if ptr == nil || data == nil || size < 0 {
		return -1
	}
	ptr.dataMutex.Lock()
	defer ptr.dataMutex.Unlock()
	bioClearRetryFlags(b)
	bData := nonCopyCString(data, size)
	//log.Infof("[ DTLS ] go_sinkWrite data %#v size %d", bData, size)
	ptr.buf = append(ptr.buf, bData...)

	return size
}

//export go_sinkWriteCtrl
func go_sinkWriteCtrl(b *C.BIO, cmd C.int, arg1 C.long, arg2 unsafe.Pointer) (rc C.long) {
	//log.Infof("[ DTLS ] go_sinkWriteCtrl called with cmd %d", cmd)
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: go_sinkWriteCtrl panic'd: %v", err)
			rc = -1
		}
	}()

	switch cmd {
	case C.BIO_CTRL_DUP, C.BIO_CTRL_FLUSH:
		return 1
	/*case C.BIO_CTRL_DGRAM_QUERY_MTU:
		log.Infof("[ DTLS ] advertizing MTU: %d", mtu)
		return mtu
	case C.BIO_CTRL_DGRAM_GET_FALLBACK_MTU:
		log.Infof("[ DTLS ] fallback MTU: %d", mtuFallback)
		return mtuFallback*/
	case C.BIO_CTRL_WPENDING:
		return writeBioPending(b)
	}

	return
}

func writeBioPending(b *C.BIO) C.long {
	ptr := loadWritePtr(b)
	if ptr == nil {
		return 0
	}
	ptr.dataMutex.Lock()
	defer ptr.dataMutex.Unlock()
	return C.long(len(ptr.buf))
}

func (b *writeBio) WriteTo(ch chan *packet.UDP, rAddr *net.UDPAddr) (rv int64, err error) {
	b.opMutex.Lock()
	defer b.opMutex.Unlock()

	// write whatever data we currently have
	b.dataMutex.Lock()
	//if len(b.buf) == 0 {
	//  b.dataMutex.Unlock()
	//  return
	//}
	//data := b.buf[0]
	data := b.buf
	b.buf = b.buf[:copy(b.buf, b.buf[len(data):])]
	if b.release_buffers && len(b.buf) == 0 {
		b.buf = nil
	}
	b.dataMutex.Unlock()

	n := len(data)
	if n == 0 {
		return 0, nil
	}

	//b.buf = b.buf[1:]
	//b.size -= n

	// If data are > to mtu, fragment it
	/*if len(data) > int(mtu) {
	  log.Infof("[ DTLS ] Fragmentation len(data) %d > int(mtu) %d", len(data), int(mtu))
	  i := 3
	  ldata := len(data)
	  for i < ldata {
	    for i <= ldata-3 {
	      if data[i] == 0x16 && data[i+1] == 0xfe && data[i+2] == 0xff {
	        break
	      }
	      i++
	    }
	    if i >= ldata-3 {
	      i = ldata
	    }
	    log.Infof("[ DTLS ] Fragmentation Fragmented packet len=%d : %#v", len(data[:i]), data[:i])
	    udpPacket := &UdpPacket {
	                   Data: data[:i],
	                   RAddr: rAddr,
	                 }
	    ch <- udpPacket
	    data = data[i:]
	    ldata = len(data)
	    i = 3
	    log.Infof("[ DTLS ] Fragmentation len(data) is now %d, i is %d", len(data), i)
	  }
	} else {*/
	select {
	case ch <- packet.NewUDPFromData(data, rAddr):
	default:
		err = errors.New("DTLS packet is not written, channel is closed/full")
		return
	}
	//}

	return int64(n), err
}

func (self *writeBio) Disconnect(b *C.BIO) {
	if loadWritePtr(b) == self {
		writeBioMapping.Delete(unsafe.Pointer(C.X_BIO_get_data(b)))
		C.X_BIO_set_data(b, nil)
	}
}

func (b *writeBio) MakeCBIO() *C.BIO {
	rv := C.X_BIO_new_write_bio()
	key := writeBioMapping.Set(unsafe.Pointer(b))
	C.X_BIO_set_data(rv, unsafe.Pointer(key))

	return rv
}

// Read BIO part

var readBioMapping = newMapping()

type readBio struct {
	dataMutex       my.Mutex
	opMutex         my.Mutex
	buf             []byte
	eof             bool
	release_buffers bool
}

func bioSetRetryRead(b *C.BIO) {
	C.X_BIO_set_flags(b, C.BIO_FLAGS_READ|C.BIO_FLAGS_SHOULD_RETRY)
}

func loadReadPtr(b *C.BIO) *readBio {
	return (*readBio)(readBioMapping.Get(unsafe.Pointer(C.X_BIO_get_data(b))))
}

//export go_sinkRead
func go_sinkRead(b *C.BIO, data *C.char, size C.int) (rc C.int) {
	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: go_sinkRead panic'd: %v", err)
			rc = -1
		}
	}()
	ptr := loadReadPtr(b)
	if ptr == nil || size < 0 {
		return -1
	}
	ptr.dataMutex.Lock()
	defer ptr.dataMutex.Unlock()
	bioClearRetryFlags(b)
	if len(ptr.buf) == 0 {
		if ptr.eof {
			return 0
		}
		bioSetRetryRead(b)
		return -1
	}
	if size == 0 || data == nil {
		return C.int(len(ptr.buf))
	}
	n := copy(nonCopyCString(data, size), ptr.buf)
	log.Infof("[ DTLS ] go_sinkRead data %#v size %d, n is %d", nonCopyCString(data, size)[:n], size, n)
	ptr.buf = ptr.buf[:copy(ptr.buf, ptr.buf[n:])]
	if ptr.release_buffers && len(ptr.buf) == 0 {
		ptr.buf = nil
	}

	return C.int(n)
}

//export go_sinkReadCtrl
func go_sinkReadCtrl(b *C.BIO, cmd C.int, arg1 C.long, arg2 unsafe.Pointer) (rc C.long) {
	//log.Infof("go_sinkReadCtrl called with cmd %d", cmd)

	defer func() {
		if err := recover(); err != nil {
			log.Infof("openssl: readBioCtrl panic'd: %v", err)
			rc = -1
		}
	}()
	switch cmd {
	case C.BIO_CTRL_PENDING:
		return readBioPending(b)
	case C.BIO_CTRL_DUP, C.BIO_CTRL_FLUSH:
		return 1
	default:
		return 0
	}
}

func readBioPending(b *C.BIO) C.long {
	ptr := loadReadPtr(b)
	if ptr == nil {
		return 0
	}
	ptr.dataMutex.Lock()
	defer ptr.dataMutex.Unlock()
	return C.long(len(ptr.buf))
}

func (b *readBio) ReadFromOnce(data []byte) (n int, err error) {
	b.opMutex.Lock()
	defer b.opMutex.Unlock()

	// make sure we have a destination that fits at least one SSL record
	b.dataMutex.Lock()
	if cap(b.buf) < len(b.buf)+SSLRecordSize {
		new_buf := make([]byte, len(b.buf), len(b.buf)+SSLRecordSize)
		copy(new_buf, b.buf)
		b.buf = new_buf
	}
	//dst_slice := b.buf
	copy(b.buf[len(b.buf):cap(b.buf)], data)
	n = len(data)
	b.buf = b.buf[:len(b.buf)+n]
	b.dataMutex.Unlock()

	/*copy(dst, data)
	  n = len(data)

	  if len(dst_slice) != len(b.buf) {
	    copy(b.buf[len(b.buf):len(b.buf)+n], dst)
	  }
	  b.buf = b.buf[:len(b.buf)+n]*/

	return n, err
}

func (b *readBio) MakeCBIO() *C.BIO {
	rv := C.X_BIO_new_read_bio()
	key := readBioMapping.Set(unsafe.Pointer(b))
	C.X_BIO_set_data(rv, unsafe.Pointer(key))
	return rv
}

func (self *readBio) Disconnect(b *C.BIO) {
	if loadReadPtr(b) == self {
		readBioMapping.Delete(unsafe.Pointer(C.X_BIO_get_data(b)))
		C.X_BIO_set_data(b, nil)
	}
}

func (b *readBio) MarkEOF() {
	b.dataMutex.Lock()
	defer b.dataMutex.Unlock()
	b.eof = true
}

type anyBio C.BIO

func asAnyBio(b *C.BIO) *anyBio { return (*anyBio)(b) }

func (b *anyBio) Read(buf []byte) (n int, err error) {
	if len(buf) == 0 {
		return 0, nil
	}
	n = int(C.X_BIO_read((*C.BIO)(b), unsafe.Pointer(&buf[0]), C.int(len(buf))))
	if n <= 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (b *anyBio) Write(buf []byte) (written int, err error) {
	if len(buf) == 0 {
		return 0, nil
	}
	n := int(C.X_BIO_write((*C.BIO)(b), unsafe.Pointer(&buf[0]),
		C.int(len(buf))))
	if n != len(buf) {
		return n, errors.New("BIO write failed")
	}
	return n, nil
}

func BioFilterSetMtu(mtuToSet C.long) {
	mtu = mtuToSet
	log.Infof("MTU set to %d", mtu)
	return
}
