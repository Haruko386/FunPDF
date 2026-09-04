package internal

import (
	"FunPDF/internal/handler"

	"github.com/gin-gonic/gin"
)

type Router struct {
	fileHandler        *handler.FileHandler
	albumHandler       *handler.AlbumHandler
	translatorHandler  *handler.TranslatorHandler
	providerHandler    *handler.ProviderHandler
	modelHandler       *handler.ModelHandler
	chatSessionHandler *handler.ChatSessionHandler
	runtimeHandler     *handler.RuntimeHandler
}

// NewRouter create a new router
func NewRouter(fileHandler *handler.FileHandler, albumHandler *handler.AlbumHandler, translatorHandler *handler.TranslatorHandler, providerHandler *handler.ProviderHandler, modelHandler *handler.ModelHandler, chatSessionHandler *handler.ChatSessionHandler) *Router {
	return NewRouterWithRuntime(fileHandler, albumHandler, translatorHandler, providerHandler, modelHandler, chatSessionHandler, handler.NewRuntimeHandler(handler.RuntimeInfo{
		Mode:     "server",
		Database: "mysql",
		CacheDir: "./Cache",
	}))
}

func NewRouterWithRuntime(fileHandler *handler.FileHandler, albumHandler *handler.AlbumHandler, translatorHandler *handler.TranslatorHandler, providerHandler *handler.ProviderHandler, modelHandler *handler.ModelHandler, chatSessionHandler *handler.ChatSessionHandler, runtimeHandler *handler.RuntimeHandler) *Router {
	return &Router{
		fileHandler:        fileHandler,
		albumHandler:       albumHandler,
		translatorHandler:  translatorHandler,
		providerHandler:    providerHandler,
		modelHandler:       modelHandler,
		chatSessionHandler: chatSessionHandler,
		runtimeHandler:     runtimeHandler,
	}
}

// Setup register all API routes
func (r *Router) Setup(e *gin.Engine) {
	api := e.Group("/api")
	{
		runtime := api.Group("/runtime")
		{
			runtime.GET("/info", r.runtimeHandler.Info)
			runtime.POST("/open-path", r.runtimeHandler.OpenPath)
			runtime.POST("/cache-dir/select", r.runtimeHandler.SelectCacheDir)
		}

		file := api.Group("/files")
		{
			file.GET("", r.fileHandler.ListFiles)
			file.POST("", r.fileHandler.UploadFile)
			file.POST("/import-path", r.fileHandler.ImportLocalPDFPath)

			file.PUT("/:file_id", r.fileHandler.AlertFile)
			file.DELETE("/:file_id", r.fileHandler.DeleteFile)

			file.GET("/:file_id/content", r.fileHandler.GetFile)
			file.GET("/:file_id/state", r.fileHandler.GetFileState)
			file.GET("/:file_id/thumbnail", r.fileHandler.GetFileThumbnail)
			file.PATCH("/:file_id/state", r.fileHandler.SaveFile)
			file.GET("/:file_id/album", r.fileHandler.ListFileAlbums)
			file.DELETE("/:file_id/cache", r.fileHandler.DeleteFileCache)
		}

		album := api.Group("/albums")
		{
			album.GET("", r.albumHandler.ListAlbums)
			album.POST("", r.albumHandler.CreateAlbum)

			album.GET("/:album_id", r.albumHandler.ListAlbumFiles)
			album.PUT("/:album_id", r.albumHandler.UpdateAlbum)
			album.DELETE("/:album_id", r.albumHandler.DeleteAlbum)

			album.POST("/:album_id/files", r.albumHandler.UploadFilesToAlbum)
			album.DELETE("/:album_id/files", r.albumHandler.DeleteFilesFromAlbum)
		}

		translator := api.Group("/translators")
		{
			translator.GET("", r.translatorHandler.ListTranslators)
			translator.POST("", r.translatorHandler.CreateTranslator)

			translator.POST("/:translator_name", r.translatorHandler.Translate)
		}

		provider := api.Group("/providers")
		{
			provider.GET("", r.providerHandler.ListProviders)
			provider.POST("", r.providerHandler.CreateProvider)

			provider.PATCH("/:provider_id", r.providerHandler.UpdateProvider)
			provider.DELETE("/:provider_id", r.providerHandler.DeleteProvider)

			provider.POST("/:provider_id/chat", r.modelHandler.ChatToModel)
			provider.GET("/:provider_id/list", r.modelHandler.ListSupportedModels)

			models := provider.Group("/:provider_id/models")
			{
				models.GET("", r.modelHandler.ListProviderModel)
				models.POST("", r.modelHandler.SaveProviderModels)
				models.DELETE("", r.modelHandler.DeleteProviderModels)
			}

			sessions := provider.Group("/:provider_id/sessions")
			{
				sessions.POST("", r.chatSessionHandler.SetupChatSession)
				sessions.DELETE("/:session_id", r.chatSessionHandler.DeleteSession)
				sessions.POST("/:session_id/messages", r.chatSessionHandler.SendMessages)
			}
		}
	}
}
