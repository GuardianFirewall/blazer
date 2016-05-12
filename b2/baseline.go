// Copyright 2016, Google
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package b2

import (
	"io"
	"time"

	"github.com/kurin/blazer/base"

	"golang.org/x/net/context"
)

// This file wraps the base package in a thin layer, for testing.  It should be
// the only file in b2 that imports base.

type b2RootInterface interface {
	authorizeAccount(context.Context, string, string) error
	transient(error) bool
	backoff(error) time.Duration
	reauth(error) bool
	reupload(error) bool
	createBucket(context.Context, string, string) (b2BucketInterface, error)
	listBuckets(context.Context) ([]b2BucketInterface, error)
}

type b2BucketInterface interface {
	name() string
	deleteBucket(context.Context) error
	getUploadURL(context.Context) (b2URLInterface, error)
	startLargeFile(ctx context.Context, name, contentType string, info map[string]string) (b2LargeFileInterface, error)
	listFileNames(context.Context, int, string) ([]b2FileInterface, string, error)
	listFileVersions(context.Context, int, string, string) ([]b2FileInterface, string, string, error)
	downloadFileByName(context.Context, string, int64, int64) (b2FileReaderInterface, error)
	hideFile(context.Context, string) (b2FileInterface, error)
}

type b2URLInterface interface {
	reload(context.Context) error
	uploadFile(context.Context, io.Reader, int, string, string, string, map[string]string) (b2FileInterface, error)
}

type b2FileInterface interface {
	name() string
	size() int64
	timestamp() time.Time
	status() string
	deleteFileVersion(context.Context) error
	getFileInfo(context.Context) (b2FileInfoInterface, error)
	listParts(context.Context, int, int) ([]b2FilePartInterface, int, error)
	compileParts(int64, map[int]string) b2LargeFileInterface
}

type b2LargeFileInterface interface {
	finishLargeFile(context.Context) (b2FileInterface, error)
	getUploadPartURL(context.Context) (b2FileChunkInterface, error)
}

type b2FileChunkInterface interface {
	reload(context.Context) error
	uploadPart(context.Context, io.Reader, string, int, int) (int, error)
}

type b2FileReaderInterface interface {
	io.ReadCloser
	stats() (int, string, string, map[string]string)
}

type b2FileInfoInterface interface {
	stats() (string, string, int64, string, map[string]string, string, time.Time) // bleck
}

type b2FilePartInterface interface {
	number() int
	sha1() string
	size() int64
}

type b2Root struct {
	b *base.B2
}

type b2Bucket struct {
	b *base.Bucket
}

type b2URL struct {
	b *base.URL
}

type b2File struct {
	b *base.File
}

type b2LargeFile struct {
	b *base.LargeFile
}

type b2FileChunk struct {
	b *base.FileChunk
}

type b2FileReader struct {
	b *base.FileReader
}

type b2FileInfo struct {
	b *base.FileInfo
}

type b2FilePart struct {
	b *base.FilePart
}

func (b *b2Root) authorizeAccount(ctx context.Context, account, key string) error {
	nb, err := base.AuthorizeAccount(ctx, account, key)
	if err != nil {
		return err
	}
	if b.b == nil {
		b.b = nb
		return nil
	}
	b.b.Update(nb)
	return nil
}

func (*b2Root) backoff(err error) time.Duration {
	if base.Action(err) != base.Retry {
		return 0
	}
	return base.Backoff(err)
}

func (*b2Root) reauth(err error) bool {
	return base.Action(err) == base.ReAuthenticate
}

func (*b2Root) reupload(err error) bool {
	return base.Action(err) == base.AttemptNewUpload
}

func (*b2Root) transient(err error) bool {
	return base.Action(err) == base.Retry
}

func (b *b2Root) createBucket(ctx context.Context, name, btype string) (b2BucketInterface, error) {
	bucket, err := b.b.CreateBucket(ctx, name, btype)
	if err != nil {
		return nil, err
	}
	return &b2Bucket{bucket}, nil
}

func (b *b2Root) listBuckets(ctx context.Context) ([]b2BucketInterface, error) {
	buckets, err := b.b.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	var rtn []b2BucketInterface
	for _, bucket := range buckets {
		rtn = append(rtn, &b2Bucket{bucket})
	}
	return rtn, err
}

func (b *b2Bucket) deleteBucket(ctx context.Context) error {
	return b.b.DeleteBucket(ctx)
}

func (b *b2Bucket) name() string {
	return b.b.Name
}

func (b *b2Bucket) getUploadURL(ctx context.Context) (b2URLInterface, error) {
	url, err := b.b.GetUploadURL(ctx)
	if err != nil {
		return nil, err
	}
	return &b2URL{url}, nil
}

func (b *b2Bucket) startLargeFile(ctx context.Context, name, ct string, info map[string]string) (b2LargeFileInterface, error) {
	lf, err := b.b.StartLargeFile(ctx, name, ct, info)
	if err != nil {
		return nil, err
	}
	return &b2LargeFile{lf}, nil
}

func (b *b2Bucket) listFileNames(ctx context.Context, count int, continuation string) ([]b2FileInterface, string, error) {
	fs, c, err := b.b.ListFileNames(ctx, count, continuation)
	if err != nil {
		return nil, "", err
	}
	var files []b2FileInterface
	for _, f := range fs {
		files = append(files, &b2File{f})
	}
	return files, c, nil
}

func (b *b2Bucket) listFileVersions(ctx context.Context, count int, nextName, nextID string) ([]b2FileInterface, string, string, error) {
	fs, name, id, err := b.b.ListFileVersions(ctx, count, nextName, nextID)
	if err != nil {
		return nil, "", "", err
	}
	var files []b2FileInterface
	for _, f := range fs {
		files = append(files, &b2File{f})
	}
	return files, name, id, nil
}

func (b *b2Bucket) downloadFileByName(ctx context.Context, name string, offset, size int64) (b2FileReaderInterface, error) {
	fr, err := b.b.DownloadFileByName(ctx, name, offset, size)
	if err != nil {
		return nil, err
	}
	return &b2FileReader{fr}, nil
}

func (b *b2Bucket) hideFile(ctx context.Context, name string) (b2FileInterface, error) {
	f, err := b.b.HideFile(ctx, name)
	if err != nil {
		return nil, err
	}
	return &b2File{f}, nil
}

func (b *b2URL) uploadFile(ctx context.Context, r io.Reader, size int, name, contentType, sha1 string, info map[string]string) (b2FileInterface, error) {
	file, err := b.b.UploadFile(ctx, r, size, name, contentType, sha1, info)
	if err != nil {
		return nil, err
	}
	return &b2File{file}, nil
}

func (b *b2URL) reload(ctx context.Context) error {
	return b.b.Reload(ctx)
}

func (b *b2File) deleteFileVersion(ctx context.Context) error {
	return b.b.DeleteFileVersion(ctx)
}

func (b *b2File) name() string {
	return b.b.Name
}

func (b *b2File) size() int64 {
	return b.b.Size
}

func (b *b2File) timestamp() time.Time {
	return b.b.Timestamp
}

func (b *b2File) status() string {
	return b.b.Status
}

func (b *b2File) getFileInfo(ctx context.Context) (b2FileInfoInterface, error) {
	fi, err := b.b.GetFileInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &b2FileInfo{fi}, nil
}

func (b *b2File) listParts(ctx context.Context, next, count int) ([]b2FilePartInterface, int, error) {
	parts, n, err := b.b.ListParts(ctx, next, count)
	if err != nil {
		return nil, 0, err
	}
	var rtn []b2FilePartInterface
	for _, part := range parts {
		rtn = append(rtn, &b2FilePart{part})
	}
	return rtn, n, nil
}

func (b *b2File) compileParts(size int64, seen map[int]string) b2LargeFileInterface {
	return &b2LargeFile{b.b.CompileParts(size, seen)}
}

func (b *b2LargeFile) finishLargeFile(ctx context.Context) (b2FileInterface, error) {
	f, err := b.b.FinishLargeFile(ctx)
	if err != nil {
		return nil, err
	}
	return &b2File{f}, nil
}

func (b *b2LargeFile) getUploadPartURL(ctx context.Context) (b2FileChunkInterface, error) {
	c, err := b.b.GetUploadPartURL(ctx)
	if err != nil {
		return nil, err
	}
	return &b2FileChunk{c}, nil
}

func (b *b2FileChunk) reload(ctx context.Context) error {
	return b.b.Reload(ctx)
}

func (b *b2FileChunk) uploadPart(ctx context.Context, r io.Reader, sha1 string, size, index int) (int, error) {
	return b.b.UploadPart(ctx, r, sha1, size, index)
}

func (b *b2FileReader) Read(p []byte) (int, error) {
	return b.b.Read(p)
}

func (b *b2FileReader) Close() error {
	return b.b.Close()
}

func (b *b2FileReader) stats() (int, string, string, map[string]string) {
	return b.b.ContentLength, b.b.ContentType, b.b.SHA1, b.b.Info
}

func (b *b2FileInfo) stats() (string, string, int64, string, map[string]string, string, time.Time) {
	return b.b.Name, b.b.SHA1, b.b.Size, b.b.ContentType, b.b.Info, b.b.Status, b.b.Timestamp
}

func (b *b2FilePart) number() int  { return b.b.Number }
func (b *b2FilePart) sha1() string { return b.b.SHA1 }
func (b *b2FilePart) size() int64  { return b.b.Size }
