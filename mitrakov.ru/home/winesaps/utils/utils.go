package utils

import "fmt"
import "time"
import "reflect"
import "math/rand"
import "encoding/base64"
import "golang.org/x/crypto/scrypt"

// init is a side effect Golang function (https://golang.org/doc/effective_go.html#init)
func init() {
    rand.Seed(time.Now().UnixNano())
}

// Assert is an assert function, that is absent in GoLang because of https://golang.org/doc/faq#assertions
func Assert(x ...interface{}) {
    for _, v := range x {
        if v == nil {
            panic("nil assertion failed")
        } else if reflect.ValueOf(v).IsNil() { // interface is a pair {type, value}, so here we check if value==nil
            panic(fmt.Errorf("%T is nil", v))
        }
    }
}

// Min returns a minimum among integer numbers "a" and "b"
func Min(a, b uint) uint {
    if a <= b {
        return a
    }
    return b
}

// MinF returns a minimum among float numbers "a" and "b"
func MinF(a, b float32) float32 {
    if a <= b {
        return a
    }
    return b
}

// Max returns a maximum among integer numbers "a" and "b"
func Max(a, b uint) uint {
    if a >= b {
        return a
    }
    return b
}

// MaxF returns a maximum among float numbers "a" and "b"
func MaxF(a, b float32) float32 {
    if a >= b {
        return a
    }
    return b
}

// Ternary is a ternary operator for byte operands. Recall that ternary operator is absent in GoLang because of
// https://golang.org/doc/faq#Does_Go_have_a_ternary_form.
// Please be careful, because all the arguments are evaluated! E.g. "b := Ternary(obj != nil, obj.value, 0)" will
// produce NullPointerException, if obj equals to NULL
func Ternary(condition bool, x, y byte) byte {
    if condition {
        return x
    }
    return y
}

// TernaryInt is a ternary operator for integer operands. Recall that ternary operator is absent in GoLang because of
// https://golang.org/doc/faq#Does_Go_have_a_ternary_form.
// Please be careful, because all the arguments are evaluated! E.g. "b := Ternary(obj != nil, obj.value, 0)" will
// produce NullPointerException, if obj equals to NULL
func TernaryInt(condition bool, x, y int) int {
    if condition {
        return x
    }
    return y
}

// RandString generates a new Random string of length "n" containing latin characters and digits.
// Taken from https://stackoverflow.com/questions/22892120
func RandString(n int) string {
    letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, n)
    for i := range b {
        b[i] = letterBytes[rand.Intn(len(letterBytes))]
    }
    return string(b)
}

// Convert converts local time "t" to UTC+0 time
func Convert(t time.Time) time.Time {
    _, offset := time.Now().Zone()
    return t.Local().Add(-1 * time.Second * time.Duration(offset))
}

// GetHash calculates crypto hash function for a given "password"/"salt" pair with BCrypt/SCrypt algorithm
func GetHash(password, salt string) string {
    // @mitrakov (2017-08-02): see note in CheckPassword()
    // key, err := bcrypt.GenerateFromPassword([]byte(password + salt), 10)
    // Check(err)
    // return string(key)
    key, err := scrypt.Key([]byte(password), []byte(salt), 4096, 4, 1, 32)
    Check(err)
    return base64.StdEncoding.EncodeToString(key)
}

// CheckPassword verifies that a given "hash" is produced by a given "password"/"salt" pair with BCrypt/SCrypt algorithm
func CheckPassword(hash, password, salt string) bool {
    // @mitrakov (2017-08-02): firstly I used bcrypt but it is computed too long (102ms)! scrypt takes:
    // 78ms with parameters: 16384, 8, 1, 32
    // 20ms with parameters:  8192, 4, 1, 32
    // 10ms with parameters:  4096, 4, 1, 32
    //
    // return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password + salt)) == nil
    return GetHash(password, salt) == hash
}
