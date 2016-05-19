package client

import (
	"fmt"
	"net"
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

*/
import "C"

type sslCTX struct {
	ctx  *C.SSL_CTX
	ssl  *C.SSL
	conn net.Conn
	ptr  unsafe.Pointer
}

var connMap map[unsafe.Pointer]*sslCTX

func init() {
	C.SSL_library_init()
	C.SSL_load_error_strings()
	//C.nrpe_openssl_init()

	connMap = make(map[unsafe.Pointer]*sslCTX)
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
	ctx := connMap[unsafe.Pointer(b)]

	if ctx == nil {
		return -1
	}

	l, err := ctx.conn.Write((*(*[1<<31 - 1]byte)(unsafe.Pointer(buf)))[:int(length)])

	if err != nil || l != int(length) {
		return -1
	}

	return C.int(l)
}

//export cBIORead
func cBIORead(b *C.BIO, buf *C.char, length C.int) C.int {
	ctx := connMap[unsafe.Pointer(b)]

	if ctx == nil {
		return -1
	}

	l, err := ctx.conn.Read((*(*[1<<31 - 1]byte)(unsafe.Pointer(buf)))[:int(length)])

	if err != nil || l != int(length) {
		return -1
	}

	return C.int(l)
}

//export cBIOCtrl
func cBIOCtrl(b *C.BIO, cmd C.int, arg1 C.long, arg2 unsafe.Pointer) C.long {

	switch cmd {
	case C.BIO_CTRL_PENDING:
		fmt.Printf("PENDING!\n")
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

func sendSSL(conn net.Conn, in []byte, out []byte) error {
	ctx := &sslCTX{conn: conn}

	defer func() {
		if ctx.ptr != nil && connMap[ctx.ptr] == ctx {
			delete(connMap, ctx.ptr)
		}
		if ctx.ssl != nil {
			C.SSL_free(ctx.ssl)
		}
		if ctx.ctx != nil {
			C.SSL_CTX_free(ctx.ctx)
		}
	}()

	meth := C.SSLv23_client_method()

	ctx.ctx = C.SSL_CTX_new(meth)

	if ctx.ctx == nil {
		return goifyError("nrpe: cannot create ssl context")
	}

	// disable ssl2 and ssl3
	C.SSL_CTX_set_options_func(ctx.ctx, C.SSL_OP_NO_SSLv2|C.SSL_OP_NO_SSLv3)

	ctx.ssl = C.SSL_new(ctx.ctx)

	if ctx.ssl == nil {
		return goifyError("nrpe: cannot create ssl")
	}

	// nrpe supports only Anonymous DH cipher suites
	C.SSL_CTX_set_cipher_list(ctx.ctx, C.CString("ADH"))

	b := C.BIO_new(C.BIO_s_go_conn())

	if b == nil {
		return goifyError("nrpe: cannot create BIO")
	}

	ctx.ptr = unsafe.Pointer(b)
	b.ptr = ctx.ptr

	// we can't pass GO pointer to C,
	// so we will keep ctx in global map
	// and use it to access ctx from
	// callback functions
	connMap[ctx.ptr] = ctx

	C.SSL_set_bio(ctx.ssl, b, b)

	C.SSL_set_connect_state(ctx.ssl)

	if C.SSL_do_handshake(ctx.ssl) != 1 {
		return goifyError("nrpe: error on handshake")
	}

	inLen := C.int(len(in))

	if C.SSL_write(ctx.ssl, unsafe.Pointer(&in[0]), inLen) != inLen {
		return goifyError("nrpe: error while writing")
	}

	outLen := C.int(len(out))

	if C.SSL_read(ctx.ssl, unsafe.Pointer(&out[0]), outLen) != outLen {
		return goifyError("nrpe: error while reading")
	}

	return nil
}
