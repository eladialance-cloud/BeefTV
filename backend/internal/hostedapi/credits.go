package hostedapi

import (
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// CreditService is the slice of internal/credits the HTTP surface needs.
// Keeping it an interface lets the routes be tested without a database.
type CreditService interface {
	Account(userID string) (*model.CreditAccount, error)
	Accounts(limit int) ([]model.CreditAccount, error)
	Ledger(userID string, limit int) ([]model.CreditLedgerEntry, error)
	Adjust(userID string, delta int64, operatorID string, reason string, idempotencyKey string) (*credits.Result, error)
}

// CreditDirectory is the read-only account + user lookup the admin console
// needs to show who owns a wallet. The GORM repository satisfies it.
type CreditDirectory interface {
	UserByID(id string) (*model.User, bool, error)
}

type creditWalletView struct {
	UserID string `json:"userId"`
	// Balance, Frozen and Available are fixed-point microcredits.
	Balance   int64 `json:"balance"`
	Frozen    int64 `json:"frozen"`
	Available int64 `json:"available"`
	Version   int64 `json:"version"`
}

type creditAccountView struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	Balance     int64  `json:"balance"`
	Frozen      int64  `json:"frozen"`
	Available   int64  `json:"available"`
}

func walletView(account *model.CreditAccount) creditWalletView {
	return creditWalletView{
		UserID:    account.UserID,
		Balance:   account.Balance,
		Frozen:    account.Frozen,
		Available: account.Available(),
		Version:   account.Version,
	}
}

// RegisterCreditRoutes mounts the user-facing wallet surface: balance and
// ledger. It must be wired only in the server profile; the desktop API test
// asserts these paths never appear on cmd/desktop.
func RegisterCreditRoutes(r *gin.RouterGroup, svc CreditService, resolve ActorResolver) {
	group := r.Group("/credits")

	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		return user, true
	}

	group.GET("/wallet", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		account, err := svc.Account(user.ID)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"account": walletView(account)})
	})

	group.GET("/ledger", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		limit, err := queryLimit(c, 50)
		if err != nil {
			writeError(c, err)
			return
		}
		items, err := svc.Ledger(user.ID, limit)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})
}

// RegisterCreditAdminRoutes mounts the admin console: list wallets and adjust
// points by hand. Every mutation is written to the immutable ledger with the
// acting operator and a mandatory reason.
func RegisterCreditAdminRoutes(r *gin.RouterGroup, svc CreditService, directory CreditDirectory, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/credits")

	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if err := requireAdmin(user); err != nil {
			writeError(c, err)
			return nil, false
		}
		return user, true
	}

	group.GET("/accounts", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		limit, err := queryLimit(c, 100)
		if err != nil {
			writeError(c, err)
			return
		}
		accounts, err := svc.Accounts(limit)
		if err != nil {
			writeError(c, err)
			return
		}
		views := make([]creditAccountView, 0, len(accounts))
		for index := range accounts {
			account := accounts[index]
			view := creditAccountView{
				UserID:    account.UserID,
				Balance:   account.Balance,
				Frozen:    account.Frozen,
				Available: account.Available(),
			}
			if directory != nil {
				if user, found, lookupErr := directory.UserByID(account.UserID); lookupErr == nil && found {
					view.Username = user.Username
					view.DisplayName = user.DisplayName
					view.Status = string(user.Status)
				}
			}
			views = append(views, view)
		}
		writeOK(c, gin.H{"items": views})
	})

	group.POST("/adjust", func(c *gin.Context) {
		operator, ok := actor(c)
		if !ok {
			return
		}
		var body struct {
			UserID             string `json:"userId"`
			Amount             int64  `json:"amount"`
			AmountMicrocredits *int64 `json:"amountMicrocredits"`
			Reason             string `json:"reason"`
			IdempotencyKey     string `json:"idempotencyKey"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.NewAppError(kernel.CodeInvalidArgument, "积分调整参数格式错误"))
			return
		}
		target := strings.TrimSpace(body.UserID)
		if target == "" {
			writeError(c, kernel.NewAppError(kernel.CodeInvalidArgument, "缺少目标用户"))
			return
		}
		amount := credits.FromWholeCredits(body.Amount)
		if body.AmountMicrocredits != nil {
			amount = *body.AmountMicrocredits
		}
		result, err := svc.Adjust(target, amount, operator.ID, body.Reason, body.IdempotencyKey)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{
			"entry":   result.Entry,
			"account": walletView(&result.Account),
			"applied": result.Applied,
		})
	})
}

func queryLimit(c *gin.Context, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, kernel.NewAppError(kernel.CodeInvalidArgument, "limit 必须是正整数")
	}
	return parsed, nil
}
