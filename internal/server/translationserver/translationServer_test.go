package translationserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/translate"
	"google.golang.org/protobuf/proto"
)

type answerFunc func(context.Context, translate.Request) (string, error)

func (f answerFunc) Translate(ctx context.Context, req translate.Request) (translate.Answer, error) {
	text, err := f(ctx, req)

	return translate.Answer{Text: text, Model: "test-model"}, err
}

func TestTranslationServers(t *testing.T) {
	ctx, db := testutil.Setup(t)
	t.Cleanup(func() { db.Exec(`DELETE FROM translation_servers`) })

	box, _ := secret.New("test-secret")

	var tested []translate.Endpoint

	server := &Server{db: db, box: box, newTranslator: func(endpoint translate.Endpoint) translate.Translator {
		tested = append(tested, endpoint)

		return answerFunc(func(_ context.Context, req translate.Request) (string, error) {
			if endpoint.BaseURL == "http://down:8080/v1" {
				return "", errors.New("connection refused")
			}

			return "Änderungen speichern", nil
		})
	}}

	create := func(req *pb.CreateTranslationServerRequest) (*pb.TranslationServer, error) {
		res, err := server.CreateTranslationServer(ctx, connect.NewRequest(req))
		if err != nil {
			return nil, err
		}

		return res.Msg.TranslationServer, nil
	}
	stored := func(t *testing.T, id string) models.TranslationServer {
		t.Helper()

		var row models.TranslationServer
		if err := db.First(&row, "id = ?", id).Error; err != nil {
			t.Fatalf("load %s: %v", id, err)
		}

		return row
	}

	t.Run("creates a server and stores its key encrypted", func(t *testing.T) {
		created, err := create(&pb.CreateTranslationServerRequest{
			Name: " Laptop ", BaseUrl: "http://laptop:8080/v1", Model: "hy-mt2", ApiKey: "sk-123", Enabled: true,
		})
		if err != nil {
			t.Fatalf("CreateTranslationServer: %v", err)
		}

		if created.Name != "Laptop" || !created.HasApiKey || created.MaxConcurrency != 2 || !created.Enabled {
			t.Errorf("created: got %+v", created)
		}

		row := stored(t, created.Id)
		if row.APIKey == "sk-123" || row.APIKey == "" {
			t.Errorf("stored key: got %q", row.APIKey)
		}

		if plain, err := box.Decrypt(row.APIKey); err != nil || plain != "sk-123" {
			t.Errorf("decrypted key: got %q, %v", plain, err)
		}
	})

	t.Run("keeps a server disabled when asked", func(t *testing.T) {
		created, err := create(&pb.CreateTranslationServerRequest{Name: "Spare", BaseUrl: "https://spare.example/v1"})
		if err != nil {
			t.Fatalf("CreateTranslationServer: %v", err)
		}

		if created.Enabled || stored(t, created.Id).Enabled {
			t.Errorf("expected the server to stay disabled")
		}
	})

	t.Run("rejects invalid settings", func(t *testing.T) {
		cases := []*pb.CreateTranslationServerRequest{
			{BaseUrl: "http://laptop:8080/v1"},
			{Name: "No URL"},
			{Name: "Relative", BaseUrl: "/v1"},
			{Name: "Other scheme", BaseUrl: "ftp://laptop/v1"},
			{Name: "Too many", BaseUrl: "http://laptop/v1", MaxConcurrency: 65},
		}
		for _, req := range cases {
			if _, err := create(req); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("%+v: got %v", req, err)
			}
		}
	})

	t.Run("needs an encryption key to store an API key", func(t *testing.T) {
		noKey, _ := secret.New("")
		withoutKey := &Server{db: db, box: noKey}

		_, err := withoutKey.CreateTranslationServer(ctx, connect.NewRequest(&pb.CreateTranslationServerRequest{
			Name: "Hosted", BaseUrl: "https://api.example/v1", ApiKey: "sk-123",
		}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("got %v", err)
		}
		// Without a key there is nothing to encrypt.
		if _, err := withoutKey.CreateTranslationServer(ctx, connect.NewRequest(&pb.CreateTranslationServerRequest{
			Name: "Local", BaseUrl: "http://llama:8080/v1",
		})); err != nil {
			t.Errorf("without a key: %v", err)
		}
	})

	t.Run("lists servers without their keys", func(t *testing.T) {
		res, err := server.ListTranslationServers(ctx, connect.NewRequest(&pb.ListTranslationServersRequest{}))
		if err != nil {
			t.Fatalf("ListTranslationServers: %v", err)
		}

		if len(res.Msg.TranslationServers) < 2 || res.Msg.TranslationServers[0].Name != "Laptop" {
			t.Errorf("got %+v", res.Msg.TranslationServers)
		}

		for _, item := range res.Msg.TranslationServers {
			if text := item.String(); strings.Contains(text, "sk-123") || strings.Contains(text, "v1:") {
				t.Errorf("listed a key: %v", item)
			}
		}
	})

	t.Run("updates only the given settings", func(t *testing.T) {
		created, _ := create(&pb.CreateTranslationServerRequest{Name: "Homelab", BaseUrl: "http://homelab:8080/v1", ApiKey: "sk-123", Enabled: true})

		res, err := server.UpdateTranslationServer(ctx, connect.NewRequest(&pb.UpdateTranslationServerRequest{
			Id: created.Id, MaxConcurrency: proto.Int32(4), Enabled: proto.Bool(false),
		}))
		if err != nil {
			t.Fatalf("UpdateTranslationServer: %v", err)
		}

		updated := res.Msg.TranslationServer
		if updated.MaxConcurrency != 4 || updated.Enabled || updated.Name != "Homelab" || !updated.HasApiKey {
			t.Errorf("updated: got %+v", updated)
		}

		res, err = server.UpdateTranslationServer(ctx, connect.NewRequest(&pb.UpdateTranslationServerRequest{
			Id: created.Id, ApiKey: proto.String(""),
		}))
		if err != nil || res.Msg.TranslationServer.HasApiKey || stored(t, created.Id).APIKey != "" {
			t.Errorf("removing the key: got %+v, %v", res.Msg.GetTranslationServer(), err)
		}

		_, err = server.UpdateTranslationServer(ctx, connect.NewRequest(&pb.UpdateTranslationServerRequest{
			Id: created.Id, BaseUrl: proto.String("not a url"),
		}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("invalid url: got %v", err)
		}
	})

	t.Run("tests a server with its decrypted key", func(t *testing.T) {
		created, _ := create(&pb.CreateTranslationServerRequest{Name: "Test", BaseUrl: "http://test:8080/v1", ApiKey: "sk-test"})
		tested = nil

		res, err := server.TestTranslationServer(ctx, connect.NewRequest(&pb.TestTranslationServerRequest{Id: created.Id}))
		if err != nil {
			t.Fatalf("TestTranslationServer: %v", err)
		}

		if !res.Msg.Ok || res.Msg.Translation != "Änderungen speichern" {
			t.Errorf("got %+v", res.Msg)
		}

		if len(tested) != 1 || tested[0].APIKey != "sk-test" {
			t.Errorf("endpoint: got %+v", tested)
		}

		down, _ := create(&pb.CreateTranslationServerRequest{Name: "Down", BaseUrl: "http://down:8080/v1"})

		res, err = server.TestTranslationServer(ctx, connect.NewRequest(&pb.TestTranslationServerRequest{Id: down.Id}))
		if err != nil {
			t.Fatalf("TestTranslationServer: %v", err)
		}

		if res.Msg.Ok || res.Msg.Error != "connection refused" {
			t.Errorf("got %+v", res.Msg)
		}
	})

	t.Run("deletes a server", func(t *testing.T) {
		created, _ := create(&pb.CreateTranslationServerRequest{Name: "Old", BaseUrl: "http://old:8080/v1"})
		if _, err := server.DeleteTranslationServer(ctx, connect.NewRequest(&pb.DeleteTranslationServerRequest{Id: created.Id})); err != nil {
			t.Fatalf("DeleteTranslationServer: %v", err)
		}

		_, err := server.DeleteTranslationServer(ctx, connect.NewRequest(&pb.DeleteTranslationServerRequest{Id: created.Id}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("second delete: got %v", err)
		}
	})
}
