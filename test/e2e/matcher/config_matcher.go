/*
Copyright The KubeDB Authors.

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
package matcher

import (
	"fmt"

	"github.com/onsi/gomega/types"
)

func UseCustomConfig(config string) types.GomegaMatcher {
	return &configMatcher{
		expected: config,
	}
}

type configMatcher struct {
	expected string
}

func (matcher *configMatcher) Match(actual interface{}) (success bool, err error) {
	result := actual.(string)
	if matcher.expected == result {
		return true, nil
	}
	return false, nil
}

func (matcher *configMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v to be equivalent to %v", actual, matcher.expected)
}

func (matcher *configMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %v not to be equivalent to %v", actual, matcher.expected)
}
