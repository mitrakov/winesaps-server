package utils

import "fmt"
import "log"
import "reflect"

// Error is a extension of standard Go "error"
type Error struct /* implements error */ {
    Code   byte
    Text   string
    Origin interface{}
}

// Error returns an error description as a string
func (e *Error) Error() string {
    if e != nil {
        return fmt.Sprintf("%s (%d): %s", e.Origin, e.Code, e.Text)
    }
    return "no_err"
}

// NewErr creates a new Error instance from the scratch
// "who" - the owner component
// "code" - error code
// "txt" - error text with optional arguments like %d, %s, etc.
// "args" - arguments [optional]
func NewErr(who interface{}, code byte, txt string, args ...interface{}) *Error {
    return &Error{code, fmt.Sprintf(txt, args...), reflect.TypeOf(who)}
}

// NewErrFromError creates a new Error instance from the existing Golang error.
// "who" - the owner component
// "code" - error code
// "err" - base error
func NewErrFromError(who interface{}, code byte, err error) *Error {
    if err == nil {
        return nil
    }
    return NewErr(who, code, err.Error())
}

// NewErrs creates a new compound Error from several another Errors. The first instance is considered to be main, and
// the other ones will be appended by semicolon with corresponding codes and descriptions.
// "errs" - errors
func NewErrs(errs ...*Error) *Error {
    var res *Error
    for _, e := range errs {
        if e != nil {
            if res == nil {
                res = e
            } else {
                res.Text += fmt.Sprintf("; WITH ERROR: %s (%d)", e.Text, e.Code)
            }
        }
    }
    return res
}

// GetErrorCode returns a error code of a given Error
func GetErrorCode(err *Error) byte {
    if err != nil {
        return err.Code
    }
    return 0
}

// Check prints a given error to log, if it exists (othersise method does nothing)
func Check(err error) {
    if err != nil && !reflect.ValueOf(err).IsNil() {
        log.Println("ERROR: ", err)
    }
}
