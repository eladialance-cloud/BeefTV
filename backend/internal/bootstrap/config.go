package bootstrap

import (
	"time"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/asset"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type Profile string

const (
	ProfileServer  Profile = "server"
	ProfileDesktop Profile = "desktop"
)

type Config struct {
	Profile          Profile
	DataDir          string
	DatabaseDriver   string
	DatabaseURL      string
	ListenAddr       string
	LaunchToken      string
	AutoMigrate      bool
	ShutdownTimeout  time.Duration
	RouterMiddleware []gin.HandlerFunc
	// HostedRoutes 由服务端入口注入，用于挂载只在 hosted profile 存在的后台路由。
	// bootstrap 不直接 import 托管包，桌面二进制的依赖图才能保持干净。
	HostedRoutes func(HostedContext) error
	// HostedBlobResolver 由服务端入口注入对象存储路由。桌面形态为 nil，资源
	// 字节只落本机 FileStore；服务端形态在构造 app.Service 时注入，因为资源域
	// 一旦交给调用方就不能再换后端。
	HostedBlobResolver func(repo *repository.Repository, dataDir string) (asset.BlobResolver, error)
	// HostedRouterMiddleware 由服务端入口注入：在业务路由注册前拿到仓储与数据
	// 目录，返回只在 hosted profile 生效的全局中间件（如会话认证）。放在这里
	// 是因为 gin 的 RouterGroup.Use 不会回溯已注册的路由，中间件必须在 Group
	// 创建之前装上。
	HostedRouterMiddleware func(repo *repository.Repository, dataDir string) ([]gin.HandlerFunc, error)
}

// HostedContext 是托管路由扩展能拿到的共享运行时对象。
type HostedContext struct {
	API         *gin.RouterGroup
	Service     *app.Service
	Repository  *repository.Repository
	DataDir     string
	WorkspaceID string
}

func (c Config) withDefaults() Config {
	if c.Profile == "" {
		c.Profile = ProfileServer
	}
	if c.DatabaseDriver == "" {
		c.DatabaseDriver = "sqlite"
	}
	if c.ListenAddr == "" {
		if c.Profile == ProfileDesktop {
			c.ListenAddr = "127.0.0.1:0"
		} else {
			c.ListenAddr = ":8080"
		}
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 10 * time.Minute
	}
	return c
}
