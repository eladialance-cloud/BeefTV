# ToAPI Videos 接口字段

## 协议身份

- 插件 ID：`toapis-video`。
- Provider ID：`toapis-video`。
- 能力：`video`。
- 创建：`POST /v1/videos/generations`。
- 查询：`GET /v1/videos/generations/{{taskId}}`。

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | ToAPI API Key。 |

## 统一字段

| 字段 | 类型 | 上游字段 |
| --- | --- | --- |
| `model` | string | `model` |
| `prompt` | string | `prompt` |
| `duration` | integer | `duration` |
| `aspectRatio` | string | `aspect_ratio` |
| `resolution` | string | `resolution` |
| `images` | media[] | `image_with_roles` |
| `videos` | media[] | `video_with_roles` |
| `audios` | media[] | `audio_with_roles` |
| `generateAudio` | boolean | `generate_audio` |
| `watermark` | boolean | `watermark` |

## 媒体角色

- 图片：`first_frame`、`last_frame`、`reference_image`。
- 视频：`reference_video`。
- 音频：`reference_audio`。

## 响应

- 创建和查询返回 `id`、`status`、`progress`。
- 完成后从 `result.data[].url` 读取视频地址。
- 失败原因从 `error.message` 读取。

## 兼容边界

Base URL 必须指向 ToAPI 根地址，例如 `https://toapis.cn`。不要填写成完整接口路径。

<!-- BEEFTV_PLUGIN_MANIFEST_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "beeftv.plugin/v2",
  "id": "toapis-video",
  "name": "ToAPI Videos",
  "version": "1.0.0",
  "author": "BeefTV Contributors",
  "description": "ToAPI 统一异步视频生成协议。",
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "API Key",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "toapis-video",
        "label": "ToAPI Videos",
        "capabilities": [
          "video"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://toapis.cn",
        "requiresPublicMediaUrls": true,
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "视频模型 ID。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": false,
            "mapping": "prompt",
            "description": "视频提示词。"
          },
          {
            "name": "images",
            "type": "media[]",
            "required": false,
            "mapping": "image_with_roles",
            "description": "首帧、尾帧或参考图片。"
          },
          {
            "name": "videos",
            "type": "media[]",
            "required": false,
            "mapping": "video_with_roles",
            "description": "参考视频。"
          },
          {
            "name": "audios",
            "type": "media[]",
            "required": false,
            "mapping": "audio_with_roles",
            "description": "参考音频。"
          },
          {
            "name": "duration",
            "type": "integer",
            "required": false,
            "mapping": "duration",
            "description": "视频时长秒数。"
          },
          {
            "name": "aspectRatio",
            "type": "string",
            "required": false,
            "mapping": "aspect_ratio",
            "description": "视频宽高比。"
          },
          {
            "name": "resolution",
            "type": "string",
            "required": false,
            "mapping": "resolution",
            "description": "视频分辨率。"
          },
          {
            "name": "generateAudio",
            "type": "boolean",
            "required": false,
            "mapping": "generate_audio",
            "description": "是否生成同步音频。"
          },
          {
            "name": "watermark",
            "type": "boolean",
            "required": false,
            "mapping": "watermark",
            "description": "是否添加 AIGC 水印。"
          },
          {
            "name": "providerOptions",
            "type": "object",
            "required": false,
            "mapping": "provider-specific fields",
            "description": "厂商扩展字段。"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/v1/videos/generations",
          "contentType": "application/json",
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "prompt": {
              "$omitEmpty": {
                "$ref": "request.prompt"
              }
            },
            "duration": {
              "$omitEmpty": {
                "$ref": "request.duration"
              }
            },
            "aspect_ratio": {
              "$omitEmpty": {
                "$ref": "request.aspectRatio"
              }
            },
            "resolution": {
              "$omitEmpty": {
                "$ref": "request.resolution"
              }
            },
            "image_with_roles": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.images"
                    }
                  },
                  "as": "media",
                  "in": {
                    "url": {
                      "$ref": "media.value"
                    },
                    "role": {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "media.role"
                            },
                            ""
                          ]
                        },
                        "then": "reference_image",
                        "else": {
                          "$ref": "media.role"
                        }
                      }
                    }
                  }
                }
              }
            },
            "video_with_roles": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.videos"
                    }
                  },
                  "as": "media",
                  "in": {
                    "url": {
                      "$ref": "media.value"
                    },
                    "role": "reference_video"
                  }
                }
              }
            },
            "audio_with_roles": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.audios"
                    }
                  },
                  "as": "media",
                  "in": {
                    "url": {
                      "$ref": "media.value"
                    },
                    "role": "reference_audio"
                  }
                }
              }
            },
            "generate_audio": {
              "$ref": "request.generateAudio"
            },
            "watermark": {
              "$ref": "request.watermark"
            }
          }
        },
        "poll": {
          "method": "GET",
          "path": "/v1/videos/generations/{{taskId}}",
          "contentType": "application/json"
        },
        "response": {
          "taskId": {
            "$coalesce": [
              {
                "$ref": "response.id"
              },
              {
                "$ref": "response.task_id"
              },
              {
                "$ref": "taskId"
              }
            ]
          },
          "status": {
            "$coalesce": [
              {
                "$ref": "response.status"
              },
              "pending"
            ]
          },
          "message": {
            "$coalesce": [
              {
                "$ref": "response.error.message"
              },
              {
                "$ref": "response.message"
              }
            ]
          },
          "videos": {
            "$map": {
              "from": {
                "$ref": "response.result.data"
              },
              "as": "item",
              "in": {
                "url": {
                  "$ref": "item.url"
                }
              }
            }
          },
          "resultKind": "video",
          "resultEphemeral": true,
          "errorPaths": [
            "error.code"
          ]
        }
      }
    ]
  }
}
```
<!-- BEEFTV_PLUGIN_MANIFEST_END -->
