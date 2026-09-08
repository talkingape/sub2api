package service

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type openaiOAuthClientStateStub struct {
	exchangeCalled  int32
	lastClientID    string
	lastRedirectURI string
	lastProxyURL    string
	exchangeErr     error
}

func (s *openaiOAuthClientStateStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	atomic.AddInt32(&s.exchangeCalled, 1)
	s.lastClientID = clientID
	s.lastRedirectURI = redirectURI
	s.lastProxyURL = proxyURL
	if s.exchangeErr != nil {
		return nil, s.exchangeErr
	}
	return &openai.TokenResponse{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresIn:    3600,
	}, nil
}

func (s *openaiOAuthClientStateStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *openaiOAuthClientStateStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	return s.RefreshToken(ctx, refreshToken, proxyURL)
}

func TestOpenAIOAuthService_ExchangeCode_StateRequired(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth state is required")
	require.Equal(t, int32(0), atomic.LoadInt32(&client.exchangeCalled))
}

func TestOpenAIOAuthService_ExchangeCode_StateMismatch(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
		State:     "wrong-state",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid oauth state")
	require.Equal(t, int32(0), atomic.LoadInt32(&client.exchangeCalled))
}

func TestOpenAIOAuthService_ExchangeCode_StateMatch(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	info, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
		State:     "expected-state",
	})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "at", info.AccessToken)
	require.Equal(t, openai.ClientID, info.ClientID)
	require.Equal(t, openai.ClientID, client.lastClientID)
	require.Equal(t, int32(1), atomic.LoadInt32(&client.exchangeCalled))

	_, ok := svc.sessionStore.Get("sid")
	require.False(t, ok)
}

// operatorFlow exercises URL generation too, rather than inventing session IDs.
func operatorFlow(t *testing.T, svc *OpenAIOAuthService, actor int64) *OpenAIExchangeCodeInput {
	t.Helper()
	result, err := svc.GenerateOperatorAuthURL(context.Background(), actor, nil)
	require.NoError(t, err)
	u, err := url.Parse(result.AuthURL)
	require.NoError(t, err)
	require.Equal(t, openai.DefaultRedirectURI, u.Query().Get("redirect_uri"))
	require.NotEmpty(t, u.Query().Get("state"))
	return &OpenAIExchangeCodeInput{SessionID: result.SessionID, State: u.Query().Get("state"), Code: "code"}
}

func TestOpenAIOAuthService_OperatorSessionIsolation(t *testing.T) {
	for _, scenario := range []string{"other_actor", "admin_endpoint", "state", "proxy_override", "redirect_override", "expired", "missing_actor"} {
		t.Run(scenario, func(t *testing.T) {
			client := &openaiOAuthClientStateStub{}
			svc := NewOpenAIOAuthService(nil, client)
			defer svc.Stop()
			input := operatorFlow(t, svc, 17)
			actor := int64(17)
			switch scenario {
			case "other_actor":
				actor = 18
			case "state":
				input.State = "wrong"
			case "proxy_override":
				id := int64(1)
				input.ProxyID = &id
			case "redirect_override":
				input.RedirectURI = "https://example.invalid/callback"
			case "missing_actor":
				actor = 0
			case "expired":
				session, _ := svc.sessionStore.Get(input.SessionID)
				session.CreatedAt = time.Now().Add(-openai.SessionTTL - time.Second)
			}
			var err error
			if scenario == "admin_endpoint" {
				_, err = svc.ExchangeCode(context.Background(), input)
			} else {
				_, err = svc.ExchangeOperatorCode(context.Background(), actor, input)
			}
			require.Error(t, err)
			require.Zero(t, atomic.LoadInt32(&client.exchangeCalled))
		})
	}
}

func TestOpenAIOAuthService_OperatorCannotConsumeAdminSession(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()
	svc.sessionStore.Set("admin", &openai.OAuthSession{State: "state", CreatedAt: time.Now()})
	_, err := svc.ExchangeOperatorCode(context.Background(), 17, &OpenAIExchangeCodeInput{SessionID: "admin", State: "state", Code: "code"})
	require.Error(t, err)
	require.Zero(t, atomic.LoadInt32(&client.exchangeCalled))
	_, present := svc.sessionStore.Get("admin")
	require.True(t, present)
}

func TestOpenAIOAuthService_OperatorConcurrentReplay(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()
	input := operatorFlow(t, svc, 17)
	// The trusted proxy selected when generating the session is used unchanged.
	session, _ := svc.sessionStore.Get(input.SessionID)
	session.ProxyURL = "http://selected-proxy:8080"
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.ExchangeOperatorCode(context.Background(), 17, input); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	require.Equal(t, int32(1), atomic.LoadInt32(&client.exchangeCalled))
	require.Equal(t, "http://selected-proxy:8080", client.lastProxyURL)
	require.Equal(t, openai.DefaultRedirectURI, client.lastRedirectURI)
	_, err := svc.ExchangeOperatorCode(context.Background(), 17, input)
	require.Error(t, err)
}

func TestOpenAIOAuthService_OperatorExchangeFailureConsumesSession(t *testing.T) {
	client := &openaiOAuthClientStateStub{exchangeErr: errors.New("upstream exchange failed")}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()
	input := operatorFlow(t, svc, 17)
	_, err := svc.ExchangeOperatorCode(context.Background(), 17, input)
	require.Error(t, err)
	_, err = svc.ExchangeOperatorCode(context.Background(), 17, input)
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&client.exchangeCalled))
}

func TestOpenAIOAuthService_GenerateOperatorRequiresActor(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, nil)
	defer svc.Stop()
	_, err := svc.GenerateOperatorAuthURL(context.Background(), 0, nil)
	require.Error(t, err)
}

func TestOpenAIOAuthService_OperatorProxyBinding(t *testing.T) {
	for _, scenario := range []string{"match", "changed", "omitted"} {
		t.Run(scenario, func(t *testing.T) {
			client := &openaiOAuthClientStateStub{}
			svc := NewOpenAIOAuthService(nil, client)
			defer svc.Stop()
			input := operatorFlow(t, svc, 17)
			session, _ := svc.sessionStore.Get(input.SessionID)
			selected := int64(42)
			session.ProxyID = &selected
			session.ProxyURL = "http://selected-proxy:8080"
			requested := selected
			input.ProxyID = &requested
			if scenario == "changed" {
				requested++
			}
			if scenario == "omitted" {
				input.ProxyID = nil
			}
			_, err := svc.ExchangeOperatorCode(context.Background(), 17, input)
			if scenario == "match" {
				require.NoError(t, err)
				require.Equal(t, "http://selected-proxy:8080", client.lastProxyURL)
			} else {
				require.Error(t, err)
				require.Zero(t, atomic.LoadInt32(&client.exchangeCalled))
			}
		})
	}
}
