// Copyright 2022 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sysfs_test

import (
	"os"
	"path/filepath"

	. "github.com/ironcore-dev/metalnet/encoding/sysfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sysfs", func() {
	Describe("Unmarshal", func() {
		var (
			dir1           string
			dir1UInt64File string
		)
		BeforeEach(func() {
			dir1 = filepath.Join(GinkgoT().TempDir(), "dir1")
			Expect(os.MkdirAll(dir1, 0777)).To(Succeed())

			dir1UInt64File = filepath.Join(dir1, "uint64")
			Expect(os.WriteFile(dir1UInt64File, []byte("123"), 0666)).To(Succeed())
		})

		type dir1Struct struct {
			V1 uint64 `sysfs:"uint64"`
		}

		It("should unmarshal a directory with a uint value", func() {
			s := &dir1Struct{}
			Expect(Unmarshal(dir1, s)).To(Succeed())

			Expect(s).To(Equal(&dir1Struct{
				V1: 123,
			}))
		})
	})
})
