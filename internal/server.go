package internal

import (
	"FunPDF/internal/dao"
	"FunPDF/internal/engine"
	"FunPDF/internal/entity"
	"FunPDF/internal/handler"
	"FunPDF/internal/service"
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// NewHTTPHandler init router
func NewHTTPHandler() *gin.Engine {
	return NewHTTPHandlerWithRuntime(RuntimeInfo{
		Mode:     "server",
		Database: "mysql",
		CacheDir: "./Cache",
	})
}

func NewHTTPHandlerWithCacheDir(cacheDir string) *gin.Engine {
	return NewHTTPHandlerWithRuntime(RuntimeInfo{
		Mode:     "server",
		Database: "mysql",
		CacheDir: cacheDir,
	})
}

type RuntimeInfo = handler.RuntimeInfo

func NewHTTPHandlerWithRuntime(info RuntimeInfo) *gin.Engine {
	cacheDir := info.CacheDir
	if cacheDir == "" {
		cacheDir = "./Cache"
		info.CacheDir = cacheDir
	}

	fileSrv := service.NewFileServiceWithCacheDir(cacheDir)
	fileHandler := handler.NewFileHandlerWithService(fileSrv)

	chatSessionSrv := service.NewChatSessionServiceWithCacheDir(cacheDir)
	chatSessionHandler := handler.NewChatSessionHandlerWithService(chatSessionSrv)

	albumHandler := handler.NewAlbumHandler()
	translatorHandler := handler.NewTranslatorHandler()
	providerHandler := handler.NewProviderHandler()
	modelHandler := handler.NewModelHandler()
	runtimeHandler := handler.NewRuntimeHandler(info)

	r := gin.Default()

	router := NewRouterWithRuntime(fileHandler, albumHandler, translatorHandler, providerHandler, modelHandler, chatSessionHandler, runtimeHandler)
	router.Setup(r)

	return r
}

// InitDatabaseFromEnv init DB
func InitDatabaseFromEnv() error {
	dsn := os.Getenv("FUNPDF_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:password@(127.0.0.1:3306)/funpdf?charset=utf8mb4&parseTime=True&loc=Local"
	}
	if err := dao.InitMysql(dsn); err != nil {
		log.Printf("initialize MySQL: %v", err)
		return err
	}

	return AutoMigrateDatabase()
}

// StartPDFTextCleaner
func StartPDFTextCleaner(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 30)

	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ticker.C:
				engine.PDFText.Clear()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func AutoMigrateDatabase() error {
	return dao.DB.AutoMigrate(
		&entity.File{},
		&entity.Album{},
		&entity.AlbumFile{},
		&entity.Translator{},
		&entity.Model{},
		&entity.Provider{},
		&entity.ProviderModel{},
		&entity.ChatSession{},
		&entity.Dialog{})
}

func InitSqliteDatabase(dbPath string) error {
	err := dao.InitSqlite(dbPath)
	if err != nil {
		return err
	}
	err = AutoMigrateDatabase()
	if err != nil {
		return err
	}
	return nil
}
