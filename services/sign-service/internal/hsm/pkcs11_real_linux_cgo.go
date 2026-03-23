//go:build linux && cgo

package hsm

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

typedef unsigned long CK_RV;
typedef unsigned long CK_ULONG;
typedef unsigned long CK_SLOT_ID;
typedef unsigned long CK_SESSION_HANDLE;
typedef unsigned long CK_OBJECT_HANDLE;
typedef unsigned long CK_FLAGS;
typedef unsigned long CK_ATTRIBUTE_TYPE;
typedef unsigned long CK_OBJECT_CLASS;
typedef unsigned char CK_BBOOL;
typedef unsigned char CK_BYTE;
typedef unsigned char CK_UTF8CHAR;
typedef void* CK_VOID_PTR;
typedef struct CK_VERSION {
	CK_BYTE major;
	CK_BYTE minor;
} CK_VERSION;
typedef struct CK_ATTRIBUTE {
	CK_ATTRIBUTE_TYPE type;
	CK_VOID_PTR pValue;
	CK_ULONG ulValueLen;
} CK_ATTRIBUTE;

typedef struct CK_FUNCTION_LIST CK_FUNCTION_LIST;
typedef CK_FUNCTION_LIST* CK_FUNCTION_LIST_PTR;
typedef CK_FUNCTION_LIST_PTR* CK_FUNCTION_LIST_PTR_PTR;
typedef CK_SLOT_ID* CK_SLOT_ID_PTR;
typedef CK_SESSION_HANDLE* CK_SESSION_HANDLE_PTR;
typedef CK_OBJECT_HANDLE* CK_OBJECT_HANDLE_PTR;
typedef CK_ULONG* CK_ULONG_PTR;
typedef CK_ATTRIBUTE* CK_ATTRIBUTE_PTR;
typedef CK_RV (*CK_C_GetFunctionList)(CK_FUNCTION_LIST_PTR_PTR ppFunctionList);

#define CK_TRUE 1
#define CK_FALSE 0
#define CKR_OK 0UL
#define CKR_GENERAL_ERROR 0x00000005UL
#define CKR_ATTRIBUTE_TYPE_INVALID 0x00000012UL
#define CKR_CRYPTOKI_ALREADY_INITIALIZED 0x00000191UL
#define CKR_USER_ALREADY_LOGGED_IN 0x00000100UL
#define CKF_RW_SESSION 0x00000002UL
#define CKF_SERIAL_SESSION 0x00000004UL
#define CKU_USER 1UL
#define CKO_DATA 0x00000000UL
#define CKA_CLASS 0x00000000UL
#define CKA_TOKEN 0x00000001UL
#define CKA_PRIVATE 0x00000002UL
#define CKA_LABEL 0x00000003UL
#define CKA_APPLICATION 0x00000010UL
#define CKA_VALUE 0x00000011UL
#define CK_UNAVAILABLE_INFORMATION ((CK_ULONG)-1)

struct CK_FUNCTION_LIST {
	CK_VERSION version;
	CK_RV (*C_Initialize)(CK_VOID_PTR);
	CK_RV (*C_Finalize)(CK_VOID_PTR);
	void *C_GetInfo;
	void *C_GetFunctionList;
	CK_RV (*C_GetSlotList)(CK_BBOOL tokenPresent, CK_SLOT_ID_PTR pSlotList, CK_ULONG_PTR pulCount);
	void *C_GetSlotInfo;
	void *C_GetTokenInfo;
	void *C_GetMechanismList;
	void *C_GetMechanismInfo;
	void *C_InitToken;
	void *C_InitPIN;
	void *C_SetPIN;
	CK_RV (*C_OpenSession)(CK_SLOT_ID slotID, CK_FLAGS flags, CK_VOID_PTR pApplication, CK_VOID_PTR notify, CK_SESSION_HANDLE_PTR phSession);
	CK_RV (*C_CloseSession)(CK_SESSION_HANDLE hSession);
	void *C_CloseAllSessions;
	void *C_GetSessionInfo;
	void *C_GetOperationState;
	void *C_SetOperationState;
	CK_RV (*C_Login)(CK_SESSION_HANDLE hSession, CK_ULONG userType, CK_UTF8CHAR *pPin, CK_ULONG ulPinLen);
	CK_RV (*C_Logout)(CK_SESSION_HANDLE hSession);
	CK_RV (*C_CreateObject)(CK_SESSION_HANDLE hSession, CK_ATTRIBUTE_PTR pTemplate, CK_ULONG ulCount, CK_OBJECT_HANDLE_PTR phObject);
	void *C_CopyObject;
	void *C_DestroyObject;
	void *C_GetObjectSize;
	CK_RV (*C_GetAttributeValue)(CK_SESSION_HANDLE hSession, CK_OBJECT_HANDLE hObject, CK_ATTRIBUTE_PTR pTemplate, CK_ULONG ulCount);
	void *C_SetAttributeValue;
	CK_RV (*C_FindObjectsInit)(CK_SESSION_HANDLE hSession, CK_ATTRIBUTE_PTR pTemplate, CK_ULONG ulCount);
	CK_RV (*C_FindObjects)(CK_SESSION_HANDLE hSession, CK_OBJECT_HANDLE_PTR phObject, CK_ULONG ulMaxObjectCount, CK_ULONG_PTR pulObjectCount);
	CK_RV (*C_FindObjectsFinal)(CK_SESSION_HANDLE hSession);
};

typedef struct codex_pkcs11_handle {
	void *module;
	CK_FUNCTION_LIST_PTR funcs;
} codex_pkcs11_handle;

static CK_RV codex_pkcs11_load(const char *path, codex_pkcs11_handle **out, char **errOut) {
	void *module = NULL;
	CK_C_GetFunctionList getFunctionList = NULL;
	codex_pkcs11_handle *handle = NULL;
	CK_RV rv = CKR_GENERAL_ERROR;

	if (out != NULL) {
		*out = NULL;
	}
	if (errOut != NULL) {
		*errOut = NULL;
	}
	if (path == NULL || path[0] == '\0') {
		if (errOut != NULL) {
			*errOut = strdup("pkcs11 module path is required");
		}
		return CKR_GENERAL_ERROR;
	}

	module = dlopen(path, RTLD_NOW | RTLD_LOCAL);
	if (module == NULL) {
		if (errOut != NULL) {
			const char *msg = dlerror();
			*errOut = strdup(msg != NULL ? msg : "dlopen failed");
		}
		return CKR_GENERAL_ERROR;
	}

	getFunctionList = (CK_C_GetFunctionList)dlsym(module, "C_GetFunctionList");
	if (getFunctionList == NULL) {
		if (errOut != NULL) {
			const char *msg = dlerror();
			*errOut = strdup(msg != NULL ? msg : "C_GetFunctionList symbol not found");
		}
		dlclose(module);
		return CKR_GENERAL_ERROR;
	}

	handle = (codex_pkcs11_handle *)calloc(1, sizeof(codex_pkcs11_handle));
	if (handle == NULL) {
		if (errOut != NULL) {
			*errOut = strdup("calloc failed");
		}
		dlclose(module);
		return CKR_GENERAL_ERROR;
	}

	rv = getFunctionList(&handle->funcs);
	if (rv != CKR_OK) {
		free(handle);
		dlclose(module);
		return rv;
	}
	rv = handle->funcs->C_Initialize(NULL);
	if (rv != CKR_OK && rv != CKR_CRYPTOKI_ALREADY_INITIALIZED) {
		free(handle);
		dlclose(module);
		return rv;
	}

	handle->module = module;
	if (out != NULL) {
		*out = handle;
	}
	return CKR_OK;
}

static void codex_pkcs11_unload(codex_pkcs11_handle *handle) {
	if (handle == NULL) {
		return;
	}
	if (handle->funcs != NULL && handle->funcs->C_Finalize != NULL) {
		handle->funcs->C_Finalize(NULL);
	}
	if (handle->module != NULL) {
		dlclose(handle->module);
	}
	free(handle);
}

static CK_RV codex_pkcs11_open_first_rw_session(codex_pkcs11_handle *handle, CK_SESSION_HANDLE_PTR session) {
	CK_ULONG count = 0;
	CK_SLOT_ID *slots = NULL;
	CK_RV rv = CKR_GENERAL_ERROR;

	if (handle == NULL || handle->funcs == NULL || session == NULL) {
		return CKR_GENERAL_ERROR;
	}

	rv = handle->funcs->C_GetSlotList(CK_TRUE, NULL, &count);
	if (rv != CKR_OK) {
		return rv;
	}
	if (count == 0) {
		return CKR_GENERAL_ERROR;
	}

	slots = (CK_SLOT_ID *)calloc(count, sizeof(CK_SLOT_ID));
	if (slots == NULL) {
		return CKR_GENERAL_ERROR;
	}
	rv = handle->funcs->C_GetSlotList(CK_TRUE, slots, &count);
	if (rv == CKR_OK) {
		rv = handle->funcs->C_OpenSession(slots[0], CKF_SERIAL_SESSION | CKF_RW_SESSION, NULL, NULL, session);
	}
	free(slots);
	return rv;
}

static CK_RV codex_pkcs11_login_user(codex_pkcs11_handle *handle, CK_SESSION_HANDLE session, const char *pin) {
	CK_RV rv;
	if (handle == NULL || handle->funcs == NULL) {
		return CKR_GENERAL_ERROR;
	}
	rv = handle->funcs->C_Login(session, CKU_USER, (CK_UTF8CHAR *)pin, (CK_ULONG)strlen(pin));
	if (rv == CKR_USER_ALREADY_LOGGED_IN) {
		return CKR_OK;
	}
	return rv;
}

static CK_RV codex_pkcs11_close_logged_in_session(codex_pkcs11_handle *handle, CK_SESSION_HANDLE session) {
	if (handle == NULL || handle->funcs == NULL) {
		return CKR_GENERAL_ERROR;
	}
	if (handle->funcs->C_Logout != NULL) {
		handle->funcs->C_Logout(session);
	}
	return handle->funcs->C_CloseSession(session);
}

static CK_RV codex_pkcs11_find_data_object(codex_pkcs11_handle *handle, CK_SESSION_HANDLE session, const char *application, const char *label, CK_OBJECT_HANDLE_PTR object, CK_BBOOL *found) {
	CK_OBJECT_CLASS classValue = CKO_DATA;
	CK_ULONG count = 0;
	CK_ATTRIBUTE attrs[3];
	CK_RV rv;

	if (handle == NULL || handle->funcs == NULL || object == NULL || found == NULL) {
		return CKR_GENERAL_ERROR;
	}

	attrs[0].type = CKA_CLASS;
	attrs[0].pValue = &classValue;
	attrs[0].ulValueLen = sizeof(classValue);
	attrs[1].type = CKA_APPLICATION;
	attrs[1].pValue = (CK_VOID_PTR)application;
	attrs[1].ulValueLen = (CK_ULONG)strlen(application);
	attrs[2].type = CKA_LABEL;
	attrs[2].pValue = (CK_VOID_PTR)label;
	attrs[2].ulValueLen = (CK_ULONG)strlen(label);

	rv = handle->funcs->C_FindObjectsInit(session, attrs, 3);
	if (rv != CKR_OK) {
		return rv;
	}
	rv = handle->funcs->C_FindObjects(session, object, 1, &count);
	if (rv == CKR_OK) {
		*found = count > 0 ? CK_TRUE : CK_FALSE;
	}
	{
		CK_RV finalRV = handle->funcs->C_FindObjectsFinal(session);
		if (rv == CKR_OK && finalRV != CKR_OK) {
			rv = finalRV;
		}
	}
	return rv;
}

static CK_RV codex_pkcs11_read_object_value(codex_pkcs11_handle *handle, CK_SESSION_HANDLE session, CK_OBJECT_HANDLE object, CK_BYTE **buf, CK_ULONG *length) {
	CK_ATTRIBUTE attr;
	CK_RV rv;

	if (handle == NULL || handle->funcs == NULL || buf == NULL || length == NULL) {
		return CKR_GENERAL_ERROR;
	}

	attr.type = CKA_VALUE;
	attr.pValue = NULL;
	attr.ulValueLen = 0;
	rv = handle->funcs->C_GetAttributeValue(session, object, &attr, 1);
	if (rv != CKR_OK) {
		return rv;
	}
	if (attr.ulValueLen == CK_UNAVAILABLE_INFORMATION) {
		return CKR_ATTRIBUTE_TYPE_INVALID;
	}

	*buf = (CK_BYTE *)malloc(attr.ulValueLen);
	if (*buf == NULL) {
		return CKR_GENERAL_ERROR;
	}
	attr.pValue = *buf;
	rv = handle->funcs->C_GetAttributeValue(session, object, &attr, 1);
	if (rv != CKR_OK) {
		free(*buf);
		*buf = NULL;
		return rv;
	}
	*length = attr.ulValueLen;
	return CKR_OK;
}

static void codex_pkcs11_free_buffer(CK_BYTE *buf) {
	if (buf != NULL) {
		free(buf);
	}
}

static CK_RV codex_pkcs11_store_data_object(codex_pkcs11_handle *handle, CK_SESSION_HANDLE session, const char *application, const char *label, const CK_BYTE *data, CK_ULONG dataLen) {
	CK_OBJECT_CLASS classValue = CKO_DATA;
	CK_BBOOL trueValue = CK_TRUE;
	CK_ATTRIBUTE attrs[6];
	CK_OBJECT_HANDLE object = 0;

	if (handle == NULL || handle->funcs == NULL) {
		return CKR_GENERAL_ERROR;
	}

	attrs[0].type = CKA_CLASS;
	attrs[0].pValue = &classValue;
	attrs[0].ulValueLen = sizeof(classValue);
	attrs[1].type = CKA_TOKEN;
	attrs[1].pValue = &trueValue;
	attrs[1].ulValueLen = sizeof(trueValue);
	attrs[2].type = CKA_PRIVATE;
	attrs[2].pValue = &trueValue;
	attrs[2].ulValueLen = sizeof(trueValue);
	attrs[3].type = CKA_APPLICATION;
	attrs[3].pValue = (CK_VOID_PTR)application;
	attrs[3].ulValueLen = (CK_ULONG)strlen(application);
	attrs[4].type = CKA_LABEL;
	attrs[4].pValue = (CK_VOID_PTR)label;
	attrs[4].ulValueLen = (CK_ULONG)strlen(label);
	attrs[5].type = CKA_VALUE;
	attrs[5].pValue = (CK_VOID_PTR)data;
	attrs[5].ulValueLen = dataLen;

	return handle->funcs->C_CreateObject(session, attrs, 6, &object);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

const pkcs11SeedApplication = "wallet-saas.sign-service.seed"

type realPKCS11Provider struct{}

type realPKCS11Session struct {
	mu      sync.Mutex
	handle  *C.codex_pkcs11_handle
	session C.CK_SESSION_HANDLE
	closed  bool
}

func newPlatformPKCS11Provider() PKCS11Provider {
	return &realPKCS11Provider{}
}

func (p *realPKCS11Provider) Open(cfg PKCS11Config) (PKCS11Session, error) {
	modulePath := strings.TrimSpace(cfg.ModulePath)
	if modulePath == "" {
		return nil, ErrCloudHSMNotConfigured
	}
	loginPIN := strings.TrimSpace(cfg.User) + ":" + strings.TrimSpace(cfg.PIN)

	cPath := C.CString(modulePath)
	defer C.free(unsafe.Pointer(cPath))

	var handle *C.codex_pkcs11_handle
	var detail *C.char
	rv := C.codex_pkcs11_load(cPath, &handle, &detail)
	if rv != C.CKR_OK {
		return nil, newPKCS11Error("load module", uint64(rv), detail)
	}

	cleanup := func(err error) (PKCS11Session, error) {
		C.codex_pkcs11_unload(handle)
		return nil, err
	}

	var session C.CK_SESSION_HANDLE
	if rv = C.codex_pkcs11_open_first_rw_session(handle, &session); rv != C.CKR_OK {
		return cleanup(newPKCS11Error("open session", uint64(rv), nil))
	}

	cPIN := C.CString(loginPIN)
	defer C.free(unsafe.Pointer(cPIN))
	if rv = C.codex_pkcs11_login_user(handle, session, cPIN); rv != C.CKR_OK {
		_ = C.codex_pkcs11_close_logged_in_session(handle, session)
		return cleanup(newPKCS11Error("login", uint64(rv), nil))
	}

	return &realPKCS11Session{handle: handle, session: session}, nil
}

func (s *realPKCS11Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}

	var err error
	if rv := C.codex_pkcs11_close_logged_in_session(s.handle, s.session); rv != C.CKR_OK {
		err = newPKCS11Error("close session", uint64(rv), nil)
	}
	C.codex_pkcs11_unload(s.handle)
	s.closed = true
	s.handle = nil
	return err
}

func (s *realPKCS11Session) LoadSeed(slotID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("pkcs11 session is closed")
	}

	cApp := C.CString(pkcs11SeedApplication)
	defer C.free(unsafe.Pointer(cApp))
	cLabel := C.CString(strings.TrimSpace(slotID))
	defer C.free(unsafe.Pointer(cLabel))

	var object C.CK_OBJECT_HANDLE
	var found C.CK_BBOOL
	if rv := C.codex_pkcs11_find_data_object(s.handle, s.session, cApp, cLabel, &object, &found); rv != C.CKR_OK {
		return nil, newPKCS11Error("find seed object", uint64(rv), nil)
	}
	if found != C.CK_TRUE {
		return nil, ErrPKCS11ObjectNotFound
	}

	var raw *C.CK_BYTE
	var rawLen C.CK_ULONG
	if rv := C.codex_pkcs11_read_object_value(s.handle, s.session, object, &raw, &rawLen); rv != C.CKR_OK {
		return nil, newPKCS11Error("read seed object", uint64(rv), nil)
	}
	defer C.codex_pkcs11_free_buffer(raw)

	return C.GoBytes(unsafe.Pointer(raw), C.int(rawLen)), nil
}

func (s *realPKCS11Session) StoreSeed(slotID string, seed []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("pkcs11 session is closed")
	}
	if len(seed) == 0 {
		return fmt.Errorf("seed is required")
	}

	cApp := C.CString(pkcs11SeedApplication)
	defer C.free(unsafe.Pointer(cApp))
	cLabel := C.CString(strings.TrimSpace(slotID))
	defer C.free(unsafe.Pointer(cLabel))

	if rv := C.codex_pkcs11_store_data_object(
		s.handle,
		s.session,
		cApp,
		cLabel,
		(*C.CK_BYTE)(unsafe.Pointer(&seed[0])),
		C.CK_ULONG(len(seed)),
	); rv != C.CKR_OK {
		return newPKCS11Error("store seed object", uint64(rv), nil)
	}
	return nil
}

type pkcs11Error struct {
	op     string
	rv     uint64
	detail string
}

func newPKCS11Error(op string, rv uint64, detail *C.char) error {
	msg := ""
	if detail != nil {
		msg = strings.TrimSpace(C.GoString(detail))
		C.free(unsafe.Pointer(detail))
	}
	return pkcs11Error{op: op, rv: rv, detail: msg}
}

func (e pkcs11Error) Error() string {
	name := pkcs11RVName(e.rv)
	if e.detail != "" {
		return fmt.Sprintf("pkcs11 %s failed: %s (rv=0x%x, %s)", e.op, name, e.rv, e.detail)
	}
	return fmt.Sprintf("pkcs11 %s failed: %s (rv=0x%x)", e.op, name, e.rv)
}

func pkcs11RVName(rv uint64) string {
	switch rv {
	case 0x00000000:
		return "CKR_OK"
	case 0x00000005:
		return "CKR_GENERAL_ERROR"
	case 0x00000012:
		return "CKR_ATTRIBUTE_TYPE_INVALID"
	case 0x00000100:
		return "CKR_USER_ALREADY_LOGGED_IN"
	case 0x00000191:
		return "CKR_CRYPTOKI_ALREADY_INITIALIZED"
	default:
		return "CKR_UNKNOWN"
	}
}
