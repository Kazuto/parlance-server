package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/server/permission"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Services anyone may call; the others are served behind the RBAC interceptor.
var public = map[string]bool{"SetupService": true, "AuthService": true}

func TestEveryProtectedProcedureHasARule(t *testing.T) {
	procedures := map[string]bool{}

	protoregistry.GlobalFiles.RangeFilesByPackage("parlance.v1", func(file protoreflect.FileDescriptor) bool {
		for i := 0; i < file.Services().Len(); i++ {
			service := file.Services().Get(i)
			if public[string(service.Name())] {
				continue
			}

			for j := 0; j < service.Methods().Len(); j++ {
				procedures["/"+string(service.FullName())+"/"+string(service.Methods().Get(j).Name())] = true
			}
		}

		return true
	})

	if len(procedures) == 0 {
		t.Fatal("found no procedures")
	}

	for procedure := range procedures {
		if _, ok := Rules[procedure]; !ok {
			t.Errorf("%s has no rule", procedure)
		}
	}

	for procedure := range Rules {
		if !procedures[procedure] {
			t.Errorf("rule for unknown procedure %s", procedure)
		}
	}
}

func TestAuthorize(t *testing.T) {
	_, db := testutil.Setup(t)

	tests := []struct {
		procedure string
		held      []string
		allowed   bool
	}{
		{parlancev1connect.StatsServiceGetStatsProcedure, nil, false},
		{parlancev1connect.StatsServiceGetStatsProcedure, []string{"read_entry"}, true},
		{parlancev1connect.ActivityServiceListRecentActivityProcedure, nil, false},
		{parlancev1connect.ExportServiceExportZipProcedure, []string{"read_locale"}, false},
		{parlancev1connect.ExportServiceExportZipProcedure, []string{"read_entry"}, true},
		{parlancev1connect.ImportServiceImportJsonProcedure, []string{"update_entry"}, false},
		{parlancev1connect.ImportServiceImportJsonProcedure, []string{"create_entry", "update_entry"}, true},
		{parlancev1connect.LocaleServiceListLocalesProcedure, nil, false},
		{parlancev1connect.LocaleServiceListLocalesProcedure, []string{"read_locale"}, true},
		{parlancev1connect.LocaleServiceDestroyLocaleProcedure, []string{"delete_locale"}, false},
		{parlancev1connect.LocalizationServiceSuggestTranslationsProcedure, []string{"read_entry", "update_entry"}, false},
		{parlancev1connect.LocalizationServiceSuggestTranslationsProcedure, []string{"use_machine_translation"}, false},
		{parlancev1connect.LocalizationServiceSuggestTranslationsProcedure, []string{"update_entry", "use_machine_translation"}, true},
		{parlancev1connect.LocalizationServiceBatchTranslateProcedure, []string{"update_entry"}, false},
		{parlancev1connect.TranslationServerServiceListTranslationServersProcedure, []string{"read_entry"}, false},
		{parlancev1connect.TranslationServerServiceTestTranslationServerProcedure, []string{"read_translation_server"}, false},
		{parlancev1connect.TerminologyServiceGetDefinitionHistoryProcedure, []string{"read_terminology"}, false},
		{parlancev1connect.TerminologyServiceSuggestTerminologiesProcedure, []string{"read_terminology"}, true},
		{parlancev1connect.ScopeServiceGetEntryScopesProcedure, []string{"read_entry"}, false},
		{"/parlance.v1.UnknownService/Call", []string{"read_entry"}, false},
	}
	for _, tt := range tests {
		ctx, err := testutil.ContextWithPermissions(db, tt.held...)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}

		_, err = Authorize(ctx, db, tt.procedure)
		if allowed := err == nil; allowed != tt.allowed {
			t.Errorf("%s with %v: allowed = %v, want %v (%v)", tt.procedure, tt.held, allowed, tt.allowed, err)
		} else if !allowed && connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s with %v: got %v, want permission denied", tt.procedure, tt.held, err)
		}
	}
}

func TestInterceptorGuardsServedProcedures(t *testing.T) {
	_, db := testutil.Setup(t)

	reader, err := testutil.ContextWithPermissions(db, "read_permission")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	nobody, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Stand in for the auth middleware: take the user from the context the test sends.
	path, handler := parlancev1connect.NewPermissionServiceHandler(permission.NewServer(db), connect.WithInterceptors(Interceptor(db)))
	users := map[string]context.Context{"reader": reader, "nobody": nobody}
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(users[r.Header.Get("X-Test-User")]))
	}))

	server := httptest.NewServer(mux)
	defer server.Close()

	client := parlancev1connect.NewPermissionServiceClient(server.Client(), server.URL)
	call := func(user string) error {
		req := connect.NewRequest(&pb.ListPermissionsRequest{})
		req.Header().Set("X-Test-User", user)
		_, err := client.ListPermissions(context.Background(), req)

		return err
	}

	if err := call("nobody"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("without read_permission: got %v, want permission denied", err)
	}

	if err := call("reader"); err != nil {
		t.Errorf("with read_permission: %v", err)
	}
}

func TestDeletedRolesGrantNothing(t *testing.T) {
	_, db := testutil.Setup(t)

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := Authorize(ctx, db, parlancev1connect.StatsServiceGetStatsProcedure); err != nil {
		t.Fatalf("before deleting the role: %v", err)
	}

	// ContextWithPermissions gives the user and its role the same ID.
	if err := db.Exec("UPDATE roles SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", middleware.UserID(ctx)).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := Authorize(ctx, db, parlancev1connect.StatsServiceGetStatsProcedure); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("after deleting the role: got %v, want permission denied", err)
	}
}
