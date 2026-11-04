package rbac

import (
	"billionmail-core/internal/consts"
	"billionmail-core/internal/service/public"
	"billionmail-core/internal/service/tenancy"
	"context"
	"fmt"
	"github.com/gogf/gf/v2/util/gconv"
	"strings"
	"time"

	"github.com/gogf/gf/util/guid"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTCustomClaims defines the custom claims for JWT
type JWTCustomClaims struct {
	AccountId         int64    `json:"accountId"`
	UserID            string   `json:"userId,omitempty"`
	CompanyID         string   `json:"companyId,omitempty"`
	MembershipID      string   `json:"membershipId,omitempty"`
	SessionID         string   `json:"sessionId,omitempty"`
	MembershipVersion int64    `json:"membershipVersion,omitempty"`
	CompanyRole       string   `json:"companyRole,omitempty"`
	Username          string   `json:"username"`
	Roles             []string `json:"roles"`
	ApiToken          bool     `json:"apiToken"`
	jwt.RegisteredClaims
}

// JWTService provides methods for JWT operations
type JWTService struct {
	Secret        string        // Secret key for signing JWT
	AccessExpiry  time.Duration // Duration for access token expiry
	RefreshExpiry time.Duration // Duration for refresh token expiry
}

// newJWTService creates a new JWTService instance
func newJWTService() *JWTService {
	defaultJwtSecret := ""

	if dbpass, err := public.DockerEnv("DBPASS"); err == nil {
		defaultJwtSecret += dbpass
	}

	if redispass, err := public.DockerEnv("REDISPASS"); err == nil {
		defaultJwtSecret += redispass
	}

	if defaultJwtSecret == "" {
		panic("no default jwt secret found")
	}

	return &JWTService{
		Secret:        g.Cfg().MustGet(context.Background(), "jwt.secret", defaultJwtSecret).String(),
		AccessExpiry:  time.Duration(g.Cfg().MustGet(context.Background(), "jwt.accessExpiry", 900).Int()) * time.Second,
		RefreshExpiry: time.Duration(g.Cfg().MustGet(context.Background(), "jwt.refreshExpiry", 86400*7).Int()) * time.Second,
	}
}

// GenerateTenantToken creates a short-lived access token bound to one active
// membership and one revocable refresh session.
func (s *JWTService) GenerateTenantToken(accountID int64, username string, roles []string, tenant tenancy.TenantContext, sessionID uuid.UUID) (string, int64, error) {
	expiryTime := time.Now().Add(s.AccessExpiry)
	claims := &JWTCustomClaims{
		AccountId: accountID, UserID: tenant.UserID.String(), CompanyID: tenant.CompanyID.String(),
		MembershipID: tenant.MembershipID.String(), SessionID: sessionID.String(),
		MembershipVersion: tenant.MembershipVersion, CompanyRole: tenant.Role,
		Username: username, Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime), IssuedAt: jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()), Issuer: consts.DEFAULT_SERVER_NAME,
			Subject: "access_token", ID: guid.S() + guid.S(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.Secret))
	if err != nil {
		return "", 0, err
	}
	return signedToken, expiryTime.Unix(), nil
}

// GenerateToken generates a new JWT token
func (s *JWTService) GenerateToken(accountId int64, username string, roles []string) (string, int64, error) {
	expiryTime := time.Now().Add(s.AccessExpiry)
	claims := &JWTCustomClaims{
		AccountId: accountId,
		Username:  username,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    consts.DEFAULT_SERVER_NAME,
			Subject:   "access_token",
			ID:        guid.S() + guid.S(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.Secret))
	if err != nil {
		return "", 0, err
	}

	return signedToken, expiryTime.Unix(), nil
}

// GenerateRefreshToken generates a refresh token
func (s *JWTService) GenerateRefreshToken(accountId int64, username string) (string, error) {
	expiryTime := time.Now().Add(s.RefreshExpiry)
	claims := &JWTCustomClaims{
		AccountId: accountId,
		Username:  username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    consts.DEFAULT_SERVER_NAME,
			Subject:   "refresh_token",
			ID:        guid.S() + guid.S(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.Secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// GenerateApiToken generates a JWT token for API access
func (s *JWTService) GenerateApiToken(accountId int64, username string, roles []string) (string, int64, error) {
	claims := &JWTCustomClaims{
		AccountId: accountId,
		Username:  username,
		Roles:     roles,
		ApiToken:  true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: nil,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    consts.DEFAULT_SERVER_NAME,
			Subject:   "api_token",
			ID:        guid.S() + guid.S(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.Secret))
	if err != nil {
		return "", 0, err
	}

	return signedToken, 0, nil
}

// ParseToken parses and validates a JWT token
func (s *JWTService) ParseToken(tokenString string) (*JWTCustomClaims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.ParseWithClaims(tokenString, &JWTCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.Secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(consts.DEFAULT_SERVER_NAME))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTCustomClaims); ok && token.Valid && !s.IsTokenBlacklisted(claims) {
		return claims, nil
	}

	return nil, gerror.New("invalid token")
}

// ParseTokenByRequest parses a JWT token from the request context
func (s *JWTService) ParseTokenByRequest(r *ghttp.Request) (*JWTCustomClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, gerror.New("no authorization header found")
	}
	// Parse token from Authorization header
	return s.ParseToken(strings.TrimPrefix(authHeader, "Bearer "))
}

// InvalidateToken invalidates a JWT token
func (s *JWTService) InvalidateToken(token *JWTCustomClaims) error {
	// Using redis to invalidate the token
	now := time.Now()
	if token.RegisteredClaims.ExpiresAt == nil || token.RegisteredClaims.ExpiresAt.Time.Before(now) {
		return nil
	}

	return g.Redis().SetEX(context.Background(), consts.JWT_BLACK_LIST_KEY_PREFIX+token.RegisteredClaims.ID, 0, gconv.Int64(token.RegisteredClaims.ExpiresAt.Sub(now).Seconds()))
}

// IsTokenBlacklisted checks if a token is blacklisted
func (s *JWTService) IsTokenBlacklisted(token *JWTCustomClaims) bool {
	if token.RegisteredClaims.ExpiresAt == nil {
		return false
	}

	exists, err := g.Redis().Exists(context.Background(), consts.JWT_BLACK_LIST_KEY_PREFIX+token.RegisteredClaims.ID)
	if err != nil {
		return false
	}
	return exists > 0
}

// JWTAuthMiddleware provides JWT authentication middleware
func (s *JWTService) JWTAuthMiddleware(r *ghttp.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
		r.Middleware.Next()
		return
	}
	// Skip authentication for login and refresh token endpoints
	if r.URL.Path == "/api/login" ||
		r.URL.Path == "/api/aapanel/sso" ||
		r.URL.Path == "/api/refresh-token" ||
		r.URL.Path == "/api/unsubscribe/user_group" ||
		r.URL.Path == "/api/get_validate_code" ||
		r.URL.Path == "/api/unsubscribe" ||
		r.URL.Path == "/api/unsubscribe_new" ||
		r.URL.Path == "/api/languages/set" ||
		r.URL.Path == "/api/languages/get" ||
		r.URL.Path == "/api/batch_mail/api/send" ||
		r.URL.Path == "/api/batch_mail/api/batch_send" ||
		r.URL.Path == "/api/subscribe/submit" ||
		r.URL.Path == "/api/subscribe/confirm" {
		r.Middleware.Next()
		return
	}

	resp := public.CodeMap[401]

	// no message
	resp.Msg = ""

	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		resp.Msg = "no authorization"
		r.Response.WriteJson(resp)
		r.Exit()
		return
	}

	// Parse token
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := s.ParseToken(tokenString)
	if err != nil {
		//resp.Msg = fmt.Sprintf("invalid token: %s", err.Error())
		r.Response.WriteJson(resp)
		r.Exit()
		return
	}

	// Set account info in context
	r.SetCtxVar("accountId", claims.AccountId)
	r.SetCtxVar("username", claims.Username)

	// Resolve and validate the active workspace. Legacy and API tokens are
	// attached to their first active company during the compatibility window.
	var tenant tenancy.TenantContext
	if claims.UserID != "" && claims.CompanyID != "" {
		userID, userErr := uuid.Parse(claims.UserID)
		companyID, companyErr := uuid.Parse(claims.CompanyID)
		if userErr != nil || companyErr != nil {
			r.Response.WriteJson(resp)
			r.Exit()
			return
		}
		tenant, err = tenancy.Membership(r.GetCtx(), userID, companyID)
		if err != nil || tenant.MembershipVersion != claims.MembershipVersion {
			r.Response.WriteJson(resp)
			r.Exit()
			return
		}
		if claims.SessionID != "" {
			sessionID, sessionErr := uuid.Parse(claims.SessionID)
			if sessionErr != nil {
				r.Response.WriteJson(resp)
				r.Exit()
				return
			}
			count, sessionErr := g.DB().Model("auth_sessions").Where("id", sessionID).Where("user_id", userID).WhereNull("revoked_at").WhereGT("expires_at", time.Now().UTC()).Count()
			if sessionErr != nil || count != 1 {
				r.Response.WriteJson(resp)
				r.Exit()
				return
			}
		}
	} else {
		tenant, err = tenancy.DefaultMembership(r.GetCtx(), claims.AccountId)
		if err != nil {
			r.Response.WriteJson(resp)
			r.Exit()
			return
		}
	}
	tenant.AccountID = claims.AccountId
	r.SetCtx(tenancy.WithContext(r.GetCtx(), tenant))
	r.SetCtxVar("companyId", tenant.CompanyID.String())
	r.SetCtxVar("userId", tenant.UserID.String())
	r.SetCtxVar("companyRole", tenant.Role)

	// Legacy repositories are not company-scoped yet. Fail closed for every
	// non-default workspace until an endpoint has moved under the versioned,
	// tenant-aware API surface. This keeps migrated mail data reachable only
	// from the single legacy workspace during the compatibility window.
	if !isTenantAwarePath(r.URL.Path) {
		legacyDefault, queryErr := g.DB().Model("companies").Where("id", tenant.CompanyID).Value("is_legacy_default")
		managedDeployment := strings.EqualFold(public.MustGetDockerEnv("SAAS_MANAGED_MODE", "false"), "true")
		if queryErr != nil || managedDeployment || !legacyDefault.Bool() {
			r.Response.Status = 403
			r.Response.WriteJson(g.Map{
				"code": 403, "success": false,
				"msg": "This feature is not yet available in this workspace",
			})
			r.Exit()
			return
		}
	}

	// Retrieve roles from cache or database
	cacheKey := fmt.Sprintf("ACCOUNT_ROLES_%d", claims.AccountId)
	roles := public.GetCache(cacheKey)

	if roles == nil {
		roles, err = Account().GetAccountRoles(r.GetCtx(), claims.AccountId)
		if err != nil {
			resp.Msg = "failed to get account roles"
			r.Response.WriteJson(resp)
			r.Exit()
			return
		}

		public.SetCache(cacheKey, roles, 20)
	}
	r.SetCtxVar("roles", roles)

	// Update Session
	err = r.Session.Set("SignedToken", tokenString)

	if err != nil {
		g.Log().Warning(r.GetCtx(), "Save SignedToken failed ", err)
	}

	r.Middleware.Next()
}

func isTenantAwarePath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/") || path == "/api/logout"
}
