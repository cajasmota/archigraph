// Handler definitions for the huma router fixture. After resolver
// rebind, http_endpoint_definition.source_file should land here.
package humafixture

import (
	"context"
)

func getUser(ctx context.Context, in *GetUserInput) (*GetUserOutput, error) {
	out := &GetUserOutput{}
	out.Body.ID = in.ID
	return out, nil
}

func createUser(ctx context.Context, in *CreateUserInput) (*CreateUserOutput, error) {
	out := &CreateUserOutput{}
	out.Body.ID = "u_1"
	return out, nil
}
