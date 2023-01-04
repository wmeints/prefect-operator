/*
Copyright 2023 Willem Meints.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils_test

import (
	"fmt"
	"unicode"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utils "github.com/wmeints/prefect-operator/utils"
)

var _ = Describe("Password generator", func() {
	When("A password is generated", func() {
		It("Generates the correct password", func() {
			password := utils.GeneratePassword(10, 1, 1, 1)

			Expect(len(password)).To(Equal(10))
			Expect(len(getUppercaseChars(password))).To(BeNumerically(">=", 1))
			Expect(len(getNumericChars(password))).To(BeNumerically(">=", 1))
			Expect(len(getDelimiterChars(password))).To(BeNumerically(">=", 1))
		})
	})
})

func getUppercaseChars(text string) []rune {
	var uppercaseChars []rune

	fmt.Println(text)

	for _, char := range text {
		if unicode.IsUpper(char) {
			uppercaseChars = append(uppercaseChars, char)
		}
	}

	return uppercaseChars
}

func getLowercaseChars(text string) []rune {
	var uppercaseChars []rune

	for _, char := range text {
		if unicode.IsLower(char) {
			uppercaseChars = append(uppercaseChars, char)
		}
	}

	return uppercaseChars
}

func getNumericChars(text string) []rune {
	var uppercaseChars []rune

	for _, char := range text {
		if unicode.IsNumber(char) {
			uppercaseChars = append(uppercaseChars, char)
		}
	}

	return uppercaseChars
}

func getDelimiterChars(text string) []rune {
	var uppercaseChars []rune

	for _, char := range text {
		if unicode.IsPunct(char) {
			uppercaseChars = append(uppercaseChars, char)
		}
	}

	return uppercaseChars
}
