package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

const transferBufferSize = 256 * 1024

// ProgressFunc 接收一次文件传输新增的已处理字节数。
type ProgressFunc func(int64)

// Size 递归统计远程普通文件字节数，并拒绝符号链接和特殊文件。
func (c *SFTP) Size(ctx context.Context, remotePath string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	info, err := c.client.Lstat(remotePath)
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("符号链接 %s 不支持文件操作", info.Name())
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return 0, fmt.Errorf("不支持的文件类型：%s", info.Name())
		}
		return info.Size(), nil
	}
	items, err := c.client.ReadDirContext(ctx, remotePath)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, item := range items {
		size, err := c.Size(ctx, path.Join(remotePath, item.Name()))
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

// Upload 逐层上传普通文件或目录；失败时清理本次创建的顶层目标。
func (c *SFTP) Upload(ctx context.Context, localPath, remotePath string, progress ProgressFunc) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("符号链接 %s 不支持上传", info.Name())
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", info.Name())
		}
		return c.uploadFile(ctx, localPath, remotePath, progress)
	}
	if err := c.client.Mkdir(remotePath); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = c.RemoveAll(context.Background(), remotePath, nil)
		}
	}()
	items, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = c.Upload(ctx, filepath.Join(localPath, item.Name()), path.Join(remotePath, item.Name()), progress); err != nil {
			return err
		}
	}
	return nil
}

// Download 逐层下载普通文件或目录；失败时清理本次创建的顶层目标。
func (c *SFTP) Download(ctx context.Context, remotePath, localPath string, progress ProgressFunc) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := c.client.Lstat(remotePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("符号链接 %s 不支持下载", info.Name())
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", info.Name())
		}
		return c.downloadFile(ctx, remotePath, localPath, info.Mode(), progress)
	}
	if err := os.Mkdir(localPath, info.Mode().Perm()); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(localPath)
		}
	}()
	items, err := c.client.ReadDirContext(ctx, remotePath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = c.Download(ctx, path.Join(remotePath, item.Name()), filepath.Join(localPath, item.Name()), progress); err != nil {
			return err
		}
	}
	return nil
}

// Copy 递归复制同一会话内的远程内容，不依赖服务端扩展。
func (c *SFTP) Copy(ctx context.Context, source, target string, progress ProgressFunc) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := c.client.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("符号链接 %s 不支持复制", info.Name())
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", info.Name())
		}
		return c.copyFile(ctx, source, target, progress)
	}
	if err := c.client.Mkdir(target); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = c.RemoveAll(context.Background(), target, nil)
		}
	}()
	items, err := c.client.ReadDirContext(ctx, source)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = c.Copy(ctx, path.Join(source, item.Name()), path.Join(target, item.Name()), progress); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAll 按子项到父目录的顺序递归删除远程路径，符号链接按链接本身删除。
func (c *SFTP) RemoveAll(ctx context.Context, remotePath string, progress ProgressFunc) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := c.client.Lstat(remotePath)
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		items, err := c.client.ReadDirContext(ctx, remotePath)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := c.RemoveAll(ctx, path.Join(remotePath, item.Name()), progress); err != nil {
				return err
			}
		}
		return c.client.RemoveDirectory(remotePath)
	}
	if err := c.client.Remove(remotePath); err != nil {
		return err
	}
	if info.Mode().IsRegular() && progress != nil {
		progress(info.Size())
	}
	return nil
}

// Rename 将单个远程项目重命名；调用方必须先确认目标名称不存在。
func (c *SFTP) Rename(source, target string) error {
	return c.client.Rename(source, target)
}

// Mkdir 在远程当前会话中创建单层目录。
func (c *SFTP) Mkdir(path string) error {
	return c.client.Mkdir(path)
}

func (c *SFTP) uploadFile(ctx context.Context, localPath, remotePath string, progress ProgressFunc) (err error) {
	input, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := c.client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := output.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = c.client.Remove(remotePath)
		}
	}()
	return CopyData(ctx, output, input, progress)
}

func (c *SFTP) downloadFile(ctx context.Context, remotePath, localPath string, mode os.FileMode, progress ProgressFunc) (err error) {
	input, err := c.client.Open(remotePath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		closeErr := output.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(localPath)
		}
	}()
	return CopyData(ctx, output, input, progress)
}

func (c *SFTP) copyFile(ctx context.Context, source, target string, progress ProgressFunc) (err error) {
	input, err := c.client.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := c.client.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := output.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = c.client.Remove(target)
		}
	}()
	return CopyData(ctx, output, input, progress)
}

// CopyData 复制数据，在每个数据块之间检查取消并报告已写入字节数。
func CopyData(ctx context.Context, output io.Writer, input io.Reader, progress ProgressFunc) error {
	buffer := make([]byte, transferBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			written, writeErr := output.Write(buffer[:read])
			if written > 0 && progress != nil {
				progress(int64(written))
			}
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
