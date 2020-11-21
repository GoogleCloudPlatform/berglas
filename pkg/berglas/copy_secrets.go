// Copyright 2019 The Berglas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package berglas

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

const (
	// SecretsMountPathPostfix is a postfix added to every secret mountPath.
	SecretsMountPathPostfix = "..berglas"

	// SecretsPathsEnvVarName is the name of the env var which can hold
	// list of files and directoris for Berglas to explore.
	SecretsPathsEnvVarName = "BERGLAS_SECRETS_PATHS"

	// SecretsExecUserEnvVarName is the name of the env var which can hold
	// user and group definition used by Berglas to execute the command as.
	SecretsExecUserEnvVarName = "BERGLAS_SECRETS_EXEC_USER"
)

var (
	stderr = os.Stderr
)

func (c *Client) CopySecrets(ctx context.Context, envVarContent string) error {
	if len(envVarContent) == 0 {
		return nil
	}

	dirsToWatch := make(map[string][]string)

	for _, p := range strings.Split(envVarContent, ",") {
		elems := strings.Split(p, "//")

		if len(elems) == 1 {
			if err := c.processDirectory(elems[0], ctx); err != nil {
				return errors.Wrapf(err, "failed to process directory %s", elems[0])
			}

			dirsToWatch[elems[0]] = nil
		} else if len(elems) == 2 {
			if err := c.processFile(elems[0], elems[1], ctx); err != nil {
				return errors.Wrapf(err, "failed to process file %s/%s", elems[0], elems[1])
			}

			dirsToWatch[elems[0]] = append(dirsToWatch[elems[0]], elems[1])
		}
	}

	for dir, files := range dirsToWatch {
		go c.addWatcher(dir, files, ctx)
	}

	return nil
}

func (c *Client) processDirectory(destDir string, ctx context.Context) error {
	srcDir := fmt.Sprintf("%s/%s", destDir, SecretsMountPathPostfix)

	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", srcDir)
	}

	for _, f := range files {
		if f.Mode()&os.ModeSymlink == os.ModeSymlink && !strings.HasPrefix(f.Name(), "..") {
			srcFilePath := fmt.Sprintf("%s/%s", srcDir, f.Name())
			fullDestFilePath := fmt.Sprintf("%s/%s", destDir, f.Name())

			isSecSym, err := c.isSecretSymlink(srcFilePath)
			if err != nil {
				return errors.Wrapf(err, "failed to check if file is secret symlink")
			}

			if isSecSym {
				if err := c.treatFile(srcFilePath, fullDestFilePath, ctx); err != nil {
					return errors.Wrapf(err, "failed to treat directory file")
				}
			}
		}
	}

	return nil
}

func (c *Client) processFile(destDir, filePath string, ctx context.Context) error {
	srcFilePath := fmt.Sprintf("%s/%s/%s", destDir, SecretsMountPathPostfix, filePath)
	fullDestFilePath := fmt.Sprintf("%s/%s", destDir, filePath)

	pathDirs := strings.Split(filePath, "/")

	firstSrcDirPath := fmt.Sprintf("%s/%s/%s", destDir, SecretsMountPathPostfix, pathDirs[0])

	isSecSym, err := c.isSecretSymlink(firstSrcDirPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check if first src dir is secret symlink")
	}

	if isSecSym {
		if err := c.treatFile(srcFilePath, fullDestFilePath, ctx); err != nil {
			return errors.Wrapf(err, "failed to treat file")
		}
	}

	return nil
}

func (c *Client) isSecretSymlink(file string) (bool, error) {
	s, err := os.Readlink(file)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read symlink %s", file)
	}

	if strings.HasPrefix(s, "..data") {
		return true, nil
	}

	return false, nil
}

func (c *Client) treatFile(src, dest string, ctx context.Context) error {
	isRef, ref, err := c.isContentReference(src)
	if err != nil {
		return errors.Wrapf(err, "failed to verify reference")
	}

	if isRef {
		content, err := c.Resolve(ctx, ref)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve reference")
		}

		if err := c.createFile(src, dest, string(content)); err != nil {
			return errors.Wrapf(err, "failed to create file")
		}
	} else {
		if err := c.copyFile(src, dest); err != nil {
			return errors.Wrapf(err, "failed to copy file")
		}
	}

	return nil
}

func (c *Client) isContentReference(file string) (bool, string, error) {
	f, err := os.Open(file)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to open file %s", file)
	}
	defer f.Close()

	line := ""
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line = scanner.Text()
		break
	}

	if IsSecretManagerReference(line) {
		return true, line, nil
	}

	return false, "", nil
}

func (c *Client) createFile(src, dest, content string) error {
	os.MkdirAll(path.Dir(dest), 0755)

	mode, err := c.getMode(src)
	if err != nil {
		return errors.Wrapf(err, "failed to get mode")
	}

	err = ioutil.WriteFile(dest, []byte(content), mode)
	if err != nil {
		return errors.Wrapf(err, "failed to write file")
	}

	return nil
}

func (c *Client) copyFile(src, dest string) error {
	os.MkdirAll(path.Dir(dest), 0755)

	from, err := os.Open(src)
	if err != nil {
		return errors.Wrapf(err, "failed to open src file %s", src)
	}
	defer from.Close()

	mode, err := c.getMode(src)
	if err != nil {
		return errors.Wrapf(err, "failed to get mode")
	}

	to, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return errors.Wrapf(err, "failed to open dest file %s", dest)
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return errors.Wrapf(err, "failed to copy file %s to %s", from, to)
	}

	return nil
}

func (c *Client) getMode(p string) (os.FileMode, error) {
	from, err := os.Open(p)
	if err != nil {
		return os.FileMode(int(0)), errors.Wrapf(err, "failed to open file %s", p)
	}
	defer from.Close()

	stat, err := from.Stat()
	if err != nil {
		return os.FileMode(int(0)), errors.Wrapf(err, "failed to stat file %s", p)
	}

	return stat.Mode(), nil
}

func (c *Client) addWatcher(dir string, files []string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(stderr, "failed to create new watcher for %s: %s\n", dir, err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					fmt.Fprintf(stderr, "failed to get watcher events for %s\n", dir)
					return
				}

				if event.Op&fsnotify.Create == fsnotify.Create && strings.HasSuffix(event.Name, "..data") {
					if len(files) == 0 {
						if err := c.processDirectory(dir, ctx); err != nil {
							fmt.Fprintf(stderr, "failed to process directory %s from the watcher: %s\n", dir, err)
							return
						}
					} else {
						for _, f := range files {
							if err := c.processFile(dir, f, ctx); err != nil {
								fmt.Fprintf(stderr, "failed to process file %s/%s from the watcher: %s\n", dir, f, err)
								return
							}
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					fmt.Fprintf(stderr, "failed to get watcher errors for %s: %s\n", dir, err)
					return
				}
			}
		}
	}()

	err = watcher.Add(fmt.Sprintf("%s/%s", dir, SecretsMountPathPostfix))
	if err != nil {
		fmt.Fprintf(stderr, "failed to add watcher for %s: %s\n", dir, err)
		return
	}

	<-done
}

func GetUidGid(input string) (uint32, uint32, error) {
	parts := strings.Split(input, ":")

	var usr *user.User
	// As per Docker documentation, the default group is root
	grp := user.Group{
		Gid: "0",
	}

	userByName, err := user.LookupId(parts[0])
	if err != nil {
		userById, err := user.Lookup(parts[0])
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to lookup user")
		}
		usr = userById
	} else {
		usr = userByName
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to convert uid to int")
	}

	if len(parts) == 2 {
		groupByName, err := user.LookupGroupId(parts[1])
		if err != nil {
			groupById, err := user.LookupGroup(parts[1])
			if err != nil {
				return 0, 0, errors.Wrapf(err, "failed to lookup group")
			}
			grp = *groupById
		} else {
			grp = *groupByName
		}
	}

	gid, err := strconv.Atoi(grp.Gid)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to convert gid to int")
	}

	return uint32(uid), uint32(gid), nil
}
