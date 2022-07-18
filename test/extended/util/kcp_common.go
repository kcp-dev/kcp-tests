package util

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// GetRandomString use for getting a 8 byte random string
func GetRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

// StrSliceContainsDuplicate use for checking whether string slice contains duplicate string
func StrSliceContainsDuplicate(strings []string) bool {
	elemMap := make(map[string]bool)
	for _, value := range strings {
		if _, ok := elemMap[value]; ok {
			return true
		}
		elemMap[value] = true
	}
	return false
}

// StrSliceIntersect use for none dupulicate elements slice intersect
func StrSliceIntersect(slice1, slice2 []string) []string {
	m := make(map[string]int)
	sliceResult := make([]string, 0)
	for _, value1 := range slice1 {
		m[value1]++
	}
	for _, value2 := range slice2 {
		appearTimes := m[value2]
		if appearTimes == 1 {
			sliceResult = append(sliceResult, value2)
		}
	}
	return sliceResult
}

// StrSliceToMap use for converting String Slice to Map: map[string]struct{}
func StrSliceToMap(strSlice []string) map[string]struct{} {
	set := make(map[string]struct{}, len(strSlice))
	for _, v := range strSlice {
		set[v] = struct{}{}
	}
	return set
}

// IsInMap use for checking whether the map contains specified key
func IsInMap(inputMap map[string]struct{}, inputString string) bool {
	_, ok := inputMap[inputString]
	return ok
}

// StrSliceContains use for checking whether the String Slice contains specified element, return bool
func StrSliceContains(sl []string, element string) bool {
	return IsInMap(StrSliceToMap(sl), element)
}

// StrSliceToIntSlice use for converting strings slice to integer slice
func StrSliceToIntSlice(strSlice []string) ([]int, []error) {
	var (
		intSlice = make([]int, 0, len(strSlice))
		errSlice = make([]error, 0, len(strSlice))
	)
	for _, strElement := range strSlice {
		intElement, err := strconv.Atoi(strElement)
		if err != nil {
			errSlice = append(errSlice, err)
		}
		intSlice = append(intSlice, intElement)
	}
	return intSlice, errSlice
}

// VersionIsAbove use for comparing 2 different versions
// versionA, versionB should be the same length
// E.g. [{versionA: "0.6.1", versionB: "0.5.0"}, {versionA: "0.7.0", versionB: "0.6.2"}]
// IF versionA above versionB return "bool:true"
// ELSE return "bool:false" (Contains versionA = versionB)
func VersionIsAbove(versionA, versionB string) bool {
	var (
		subVersionStringA, subVersionStringB = make([]string, 0, 5), make([]string, 0, 5)
		subVersionIntA, subVersionIntB       = make([]int, 0, 5), make([]int, 0, 5)
		errList                              = make([]error, 0, 5)
	)
	subVersionStringA = strings.Split(versionA, ".")
	subVersionIntA, errList = StrSliceToIntSlice(subVersionStringA)
	o.Expect(errList).Should(o.HaveLen(0))
	subVersionStringB = strings.Split(versionB, ".")
	subVersionIntB, errList = StrSliceToIntSlice(subVersionStringB)
	o.Expect(errList).Should(o.HaveLen(0))
	o.Expect(len(subVersionIntA)).Should(o.Equal(len(subVersionIntB)))
	var minusRes int
	for i := 0; i < len(subVersionIntA); i++ {
		minusRes = subVersionIntA[i] - subVersionIntB[i]
		if minusRes > 0 {
			e2e.Logf("Version:\"%s\" is above Version:\"%s\"", versionA, versionB)
			return true
		}
		if minusRes == 0 {
			continue
		}
		e2e.Logf("Version:\"%s\" is below Version:\"%s\"", versionA, versionB)
		return false
	}
	e2e.Logf("Version:\"%s\" is the same with Version:\"%s\"", versionA, versionB)
	return false
}

// FileExist checks whether file is exist returns bool
func FileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}
