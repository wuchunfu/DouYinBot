package service

import (
	"context"
	"fmt"
	"github.com/beego/beego/v2/client/cache"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/lifei6671/douyinbot/admin/models"
	"github.com/lifei6671/douyinbot/baidu"
	"time"
)

var (
	baiduAppId     = web.AppConfig.DefaultString("baiduappid", "")
	baiduAppKey    = web.AppConfig.DefaultString("baiduappkey", "")
	baiduSecretKey = web.AppConfig.DefaultString("baidusecretkey", "")
	baiduSignKey   = web.AppConfig.DefaultString("baidusignkey", "")
	baiduCache     = cache.NewMemoryCache()
)

func uploadBaiduNetdisk(ctx context.Context, baiduId int, filename string, remoteName string) (*baidu.CreateFile, error) {
	key := fmt.Sprintf("baidu::%d", baiduId)
	val, _ := baiduCache.Get(ctx, key)
	bd, ok := val.(*baidu.Netdisk)
	if !ok || bd == nil {
		token, err := models.NewBaiduToken().First(baiduId)
		if err != nil {
			return nil, fmt.Errorf("用户未绑定百度网盘：[baiduid=%d] - %w", baiduId, err)
		}
		bd = baidu.NewNetdisk(baiduAppId, baiduAppKey, baiduSecretKey, baiduSignKey)
		bd.SetAccessToken(&baidu.TokenResponse{
			AccessToken:          token.AccessToken,
			ExpiresIn:            token.ExpiresIn,
			RefreshToken:         token.RefreshToken,
			Scope:                token.Scope,
			CreateAt:             token.Created.Unix(),
			RefreshTokenCreateAt: token.RefreshTokenCreateAt.Unix(),
		})
		_ = bd.RefreshToken()

		_ = baiduCache.Put(ctx, key, bd, time.Duration(token.ExpiresIn)*time.Second)
	}

	uploadFile, err := baidu.NewPreCreateUploadFileParam(filename, remoteName)
	if err != nil {
		logs.Error("预创建文件失败 -> [filename=%s] ; %+v", remoteName, err)
		return nil, fmt.Errorf("预创建文件失败 -> [filename=%s] ; %w", remoteName, err)
	}
	preUploadFile, err := bd.PreCreate(uploadFile)
	if err != nil {
		logs.Error("预创建文件失败 -> [filename=%s] ; %+v", remoteName, err)
		return nil, fmt.Errorf("预创建文件失败 -> [filename=%s] ; %w", remoteName, err)
	}
	superFiles, err := bd.UploadFile(preUploadFile, remoteName)
	if err != nil {
		logs.Error("创建文件失败 -> [filename=%s] ; %+v", remoteName, err)
		return nil, fmt.Errorf("创建文件失败 -> [filename=%s] ; %w", remoteName, err)
	}
	param := baidu.NewCreateFileParam(remoteName, uploadFile.Size, false)
	param.BlockList = make([]string, len(superFiles))
	for i, f := range superFiles {
		param.BlockList[i] = f.Md5
	}
	createFile, err := bd.CreateFile(param)
	if err != nil {
		logs.Error("创建文件失败 -> [filename=%s] ; %+v", remoteName, err)
		return nil, fmt.Errorf("创建文件失败 -> [filename=%s] ; %w", remoteName, err)
	}
	return createFile, nil
}
