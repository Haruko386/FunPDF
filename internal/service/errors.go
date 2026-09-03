package service

import "errors"

var (
	ErrProviderNotFound     = errors.New("provider not found")
	ErrProviderIDRequired   = errors.New("provider id is empty")
	ErrProviderNameRequired = errors.New("provider name is empty")
	ErrProviderURLSuffix    = errors.New("provider chat and models url suffix are required")
	ErrModelNameRequired    = errors.New("model name is empty")
	ErrUnsupportedProvider  = errors.New("unsupported provider")

	ErrFileNameRequired    = errors.New("file name is empty")
	ErrFileMimeRequired    = errors.New("mime_type is empty")
	ErrAlbumNameRequired   = errors.New("album name is empty")
	ErrAlbumNotFound       = errors.New("album not found")
	ErrTranslatorNotFound  = errors.New("translators not found")
)
