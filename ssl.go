package nrpe

import (
	"fmt"
	"net"
	_ "runtime"
	"sync"
	_ "time"
	"unsafe"
)

/*
#cgo LDFLAGS: -lcrypto -lssl
#include <pthread.h>
#include <openssl/rsa.h>
#include <openssl/crypto.h>
#include <openssl/dh.h>
#include <openssl/pem.h>
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <openssl/bio.h>

// openssl multi-threading
static pthread_mutex_t *nrpe_locks;

static void nrpe_openssl_thread_callback(int mode, int index, const char * file, int line) {
    if (mode & CRYPTO_LOCK) {
        pthread_mutex_lock(&nrpe_locks[index]);
    } else {
        pthread_mutex_unlock(&nrpe_locks[index]);
    }
}

static int nrpe_openssl_init() {
    int i, j, rc, locks;

    rc = 0;

    locks = CRYPTO_num_locks();

    nrpe_locks = (pthread_mutex_t*)malloc(sizeof(pthread_mutex_t) * locks);

    if (nrpe_locks == NULL) {
        return -1;
    }

    for (i = 0; i < locks; i++) {
        rc = pthread_mutex_init(&nrpe_locks[i], NULL);
        if (rc != 0) {
            break;
        }
    }

    if (rc != 0) {
        for (j = 0; j < i; j++) {
            pthread_mutex_destroy(&nrpe_locks[i]);
        }
        free(nrpe_locks);
        nrpe_locks = NULL;
        return -1;
    }

    CRYPTO_set_locking_callback(nrpe_openssl_thread_callback);

    return 0;
}

// BIO handler

static long SSL_CTX_set_options_func(SSL_CTX *ctx, long options) {
	return SSL_CTX_set_options(ctx, options);
}

extern int cBIONew(BIO *);
extern int cBIOFree(BIO *);
extern long cBIOCtrl(BIO *b, int, long, void *);
extern int cBIOWrite(BIO *, char *, int);
extern int cBIORead(BIO *, char *, int);

static int cBIOPuts(BIO *b, const char *str) {
	return cBIOWrite(b, (char *)str, strlen(str));
}

static BIO_METHOD goConnBioMethod = {
	BIO_TYPE_SOURCE_SINK,
	"Go net.Conn BIO",
	(int (*)(BIO *, const char *, int))cBIOWrite,
	cBIORead,
	cBIOPuts,
	NULL,
	cBIOCtrl,
	cBIONew,
	cBIOFree,
	NULL
};

static BIO_METHOD *BIO_s_go_conn() {
	return &goConnBioMethod;
}

static long SSL_CTX_set_tmp_dh_func(SSL_CTX *ctx, DH *dh) {
	return SSL_CTX_set_tmp_dh(ctx, dh);
}

*/
import "C"

const (
	stateInitial     = iota
	stateInHandshake = iota
	stateReady       = iota
	stateError       = iota
)

type Conn struct {
	net.Conn
	ctx   *C.SSL_CTX
	ssl   *C.SSL
	ptr   unsafe.Pointer
	state int
}

type connectionMap struct {
	lock   sync.Mutex
	values map[unsafe.Pointer]*Conn
}

func (c *connectionMap) add(k unsafe.Pointer, v *Conn) {
	c.lock.Lock()
	c.values[k] = v
	c.lock.Unlock()
}

func (c *connectionMap) del(k unsafe.Pointer) {
	c.lock.Lock()
	delete(c.values, k)
	c.lock.Unlock()
}

func (c *connectionMap) get(k unsafe.Pointer) *Conn {
	c.lock.Lock()
	r := c.values[k]
	c.lock.Unlock()

	return r
}

var connMap connectionMap

func init() {
	C.SSL_library_init()
	C.SSL_load_error_strings()
	C.nrpe_openssl_init()

	connMap.values = make(map[unsafe.Pointer]*Conn)
}

//export cBIONew
func cBIONew(b *C.BIO) C.int {
	b.init = 1
	b.num = -1
	b.ptr = nil
	b.flags = 0
	return 1
}

//export cBIOFree
func cBIOFree(b *C.BIO) C.int {
	return 1
}

//export cBIOWrite
func cBIOWrite(b *C.BIO, buf *C.char, length C.int) C.int {
	var l int
	var err error

	defer func() {
		if e := recover(); e != nil {
			l = -1
		}
	}()

	conn := connMap.get(unsafe.Pointer(b))

	if conn == nil {
		return -1
	}

	l, err = conn.Conn.Write((*(*[1<<31 - 1]byte)(unsafe.Pointer(buf)))[:int(length)])

	if err != nil || l != int(length) {
		l = -1
	}

	return C.int(l)
}

//export cBIORead
func cBIORead(b *C.BIO, buf *C.char, length C.int) C.int {
	var l int
	var err error

	defer func() {
		if e := recover(); e != nil {
			l = -1
		}
	}()

	conn := connMap.get(unsafe.Pointer(b))

	if conn == nil {
		return -1
	}

	l, err = conn.Conn.Read((*(*[1<<31 - 1]byte)(unsafe.Pointer(buf)))[:int(length)])

	if err != nil || l != int(length) {
		l = -1
	}

	return C.int(l)
}

//export cBIOCtrl
func cBIOCtrl(b *C.BIO, cmd C.int, arg1 C.long, arg2 unsafe.Pointer) C.long {

	switch cmd {
	case C.BIO_CTRL_PENDING:
		return 0
	case C.BIO_CTRL_FLUSH, C.BIO_CTRL_DUP:
		return 1
	default:
	}

	return C.long(0)
}

func goifyError(format string, a ...interface{}) error {
	return fmt.Errorf(
		"%s: %s",
		fmt.Sprintf(format, a...),
		C.GoString(C.ERR_error_string(C.ERR_get_error(), nil)),
	)
}

func (c Conn) Clean() {
	if c.ssl != nil {
		C.SSL_free(c.ssl)
		c.ssl = nil
	}
	if c.ctx != nil {
		C.SSL_CTX_free(c.ctx)
		c.ctx = nil
	}
	if c.ptr != nil {
		connMap.del(c.ptr)
		c.ptr = nil
	}
}

func (c Conn) Close() error {
	c.Clean()
	return c.Conn.Close()
}

func (c Conn) Read(b []byte) (n int, err error) {
	if c.state == stateError {
		return 0, fmt.Errorf("nrpe: inconsistent connection state")
	}

	if c.state == stateInitial {
		c.state = stateInHandshake
		if C.SSL_do_handshake(c.ssl) != 1 {
			c.state = stateError
			return 0, goifyError("nrpe: error on ssl handshake")
		}
		c.state = stateReady
	}

	rc := int(C.SSL_read(c.ssl, unsafe.Pointer(&b[0]), C.int(len(b))))

	if rc < 0 {
		return 0, goifyError("nrpe: error while reading")
	}

	return rc, nil
}

func (c Conn) Write(b []byte) (n int, err error) {
	if c.state == stateError {
		return 0, fmt.Errorf("nrpe: inconsistent connection state")
	}

	if c.state == stateInitial {
		c.state = stateInHandshake
		if C.SSL_do_handshake(c.ssl) != 1 {
			c.state = stateError
			return 0, goifyError("nrpe: error on ssl handshake")
		}
		c.state = stateReady
	}

	rc := int(C.SSL_write(c.ssl, unsafe.Pointer(&b[0]), C.int(len(b))))

	if rc < 0 {
		return 0, goifyError("nrpe: error while writing")
	}

	return rc, nil
}

func NewSSLClient(conn net.Conn) (net.Conn, error) {
	c := &Conn{conn, nil, nil, nil, stateInitial}

	meth := C.SSLv23_client_method()

	c.ctx = C.SSL_CTX_new(meth)

	if c.ctx == nil {
		return nil, goifyError("nrpe: cannot create ssl context")
	}

	// disable ssl2 and ssl3
	C.SSL_CTX_set_options_func(c.ctx, C.SSL_OP_NO_SSLv2|C.SSL_OP_NO_SSLv3)

	// nrpe supports only Anonymous DH cipher suites
	C.SSL_CTX_set_cipher_list(c.ctx, C.CString("ADH"))

	c.ssl = C.SSL_new(c.ctx)

	if c.ssl == nil {
		return nil, goifyError("nrpe: cannot create ssl")
	}

	b := C.BIO_new(C.BIO_s_go_conn())

	if b == nil {
		return nil, goifyError("nrpe: cannot create BIO")
	}

	c.ptr = unsafe.Pointer(b)
	b.ptr = c.ptr

	// we can't pass GO pointer to C, so we will keep ctx in global map
	// and use it to access ctx from callback functions
	connMap.add(c.ptr, c)

	C.SSL_set_bio(c.ssl, b, b)

	C.SSL_set_connect_state(c.ssl)

	return c, nil
}

var dhparam *C.DH

func init() {
	dhparam = C.DH_new()

	if dhparam == nil {
		panic("cannot initialize dh")
	}

	if C.DH_generate_parameters_ex(dhparam, 512, 2, nil) != 1 {
		panic("cannot generate dh")
	}
}

func NewSSLServerConn(conn net.Conn) (net.Conn, error) {
	c := &Conn{conn, nil, nil, nil, stateInitial}

	meth := C.SSLv23_server_method()

	c.ctx = C.SSL_CTX_new(meth)

	if c.ctx == nil {
		return nil, goifyError("nrpe: cannot create ssl context")
	}

	// disable ssl2 and ssl3
	C.SSL_CTX_set_options_func(c.ctx, C.SSL_OP_NO_SSLv2|C.SSL_OP_NO_SSLv3)

	// nrpe supports only Anonymous DH cipher suites
	C.SSL_CTX_set_cipher_list(c.ctx, C.CString("ADH"))

	C.SSL_CTX_set_tmp_dh_func(c.ctx, dhparam)

	c.ssl = C.SSL_new(c.ctx)

	if c.ssl == nil {
		return nil, goifyError("nrpe: cannot create ssl")
	}

	b := C.BIO_new(C.BIO_s_go_conn())

	if b == nil {
		return nil, goifyError("nrpe: cannot create BIO")
	}

	c.ptr = unsafe.Pointer(b)
	b.ptr = c.ptr

	// we can't pass GO pointer to C, so we will keep ctx in global map
	// and use it to access ctx from callback functions
	connMap.add(c.ptr, c)

	C.SSL_set_bio(c.ssl, b, b)

	C.SSL_set_accept_state(c.ssl)

	return c, nil
}
