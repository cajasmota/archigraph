// Generated-style protoc-gen-validate output fixture (issue #3255). Each
// request message gets a Validate()/ValidateAll() method — the request_
// validation surface kratos enforces via the validate middleware.
package v1

import "github.com/go-kratos/kratos/v2"

func (m *HelloRequest) Validate() error {
	return m.validate(false)
}

func (m *CreateGreetingRequest) ValidateAll() error {
	return m.validate(true)
}
