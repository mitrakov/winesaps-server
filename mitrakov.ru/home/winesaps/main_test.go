package main

import "bytes"
import "testing"

// This is a benchmark for testing bytearray performance. Please see note#2 for more details

// BenchmarkAppendEmpty tests the most simple way: append to the end of empty bytearray
func BenchmarkAppendEmpty(b *testing.B) {
    a := []byte{}
    for i := 0; i < b.N; i++ {
        a = append(a, byte(i))
    }
}

// BenchmarkBuffer tests bytes.Buffer structure
func BenchmarkBuffer(b *testing.B) {
    a := bytes.Buffer{}
    for i := 0; i < b.N; i++ {
        a.WriteByte(byte(i))
    }
}

// BenchmarkAppendCapacity tests the case when size is 0, but capacity is N
func BenchmarkAppendCapacity(b *testing.B) {
    a := make([]byte, 0, b.N)
    for i := 0; i < b.N; i++ {
        a = append(a, byte(i))
    }
}

// BenchmarkAllocated tests the best case: when ALL memory is allocated at the start
func BenchmarkAllocated(b *testing.B) {
    b.N = 200000000 // nolint
    a := make([]byte, b.N)
    for i := 0; i < b.N; i++ {
        a[i] = byte(i)
    }
}
