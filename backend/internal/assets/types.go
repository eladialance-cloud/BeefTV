package assets

import (
	"io"

	"infinite-canvas/backend/internal/model"
)

type ResourceStream struct {
	Resource      *model.Resource
	Body          io.ReadCloser
	StatusCode    int
	ContentLength int64
	ContentRange  string
	AcceptRanges  string
}

type ResourceDeliveryOptions struct {
	ForceDirect bool
	ForceProxy  bool
}

type ResourceDelivery struct {
	Resource    *model.Resource
	Stream      *ResourceStream
	RedirectURL string
	// RedirectStable 表示 RedirectURL 是稳定的公开地址（CDN）。短时签名直连
	// 地址为 false：跳转响应绝不能长期缓存，否则浏览器会复用已过期的地址。
	RedirectStable bool
}
