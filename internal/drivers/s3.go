package drivers

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sirupsen/logrus"
	"context"
)

func init() {
	AddDriver("s3", &S3Driver{})
}

type S3Driver struct {
	BaseDriver

	bucket string
	cfg    aws.Config
}

func (d *S3Driver) Init() error {
	d.bucket = os.Getenv("GRB_S3_BUCKET")

	if d.bucket == "" {
		return fmt.Errorf("you need to set 'GRB_S3_BUCKET' for a target bucket")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load AWS SDK config, %v", err)
	}
  d.cfg = cfg
	
	svc := sts.NewFromConfig(d.cfg)
	identity, err := svc.GetCallerIdentity(context.TODO(),
		&sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}
	logrus.Debugf("Connected to s3 as %s", aws.ToString(identity.Arn))

	return nil
}

func (d *S3Driver) ListDirs(path string) ([]string, error) {
	res := []string{}

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	items, err := d.listRaw(path)
	if err != nil {
		return res, err
	}

	return s3ItemsToFolders(path, items), err
}

func s3ItemsToFolders(path string, items []string) []string {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	res := []string{}
	for _, i := range items {
		res = append(res, strings.Split(strings.TrimPrefix(i, path), "/")[0])
	}

	return removeDuplicateStr(res)
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func (d *S3Driver) listRaw(path string) ([]string, error) {
	res := []string{}

	svc := s3.NewFromConfig(d.cfg)

  paginator := s3.NewListObjectsV2Paginator(svc, &s3.ListObjectsV2Input{
		Bucket:  aws.String(d.bucket),
		Prefix:  aws.String(path),
		MaxKeys: aws.Int32(20),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return res, err
		}
		for _, i := range page.Contents {
			res = append(res, aws.ToString(i.Key))
		}
	}

	logrus.Tracef("Listing %s:%s -> %v", d.bucket, path, res)

	return res, nil
}

func (d *S3Driver) Mkdir(path string) error {
	// Not needed in s3
	return nil
}

func (d *S3Driver) Delete(src string) error {

	items, err := d.listRaw(src)
	if err != nil {
		return err
	}

	svc := s3.NewFromConfig(d.cfg)
	for _, item := range items {
		logrus.Tracef("Deleting %s:%s", d.bucket, item)
		if _, err := svc.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(d.bucket),
			Key:    aws.String(item),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (d *S3Driver) Copy(src, dst string) (int64, error) {
	uploader := manager.NewUploader(s3.NewFromConfig(d.cfg))

	f, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %q, %v", src, err)
	}

	defer f.Close()

	logrus.Tracef("Uploading %s to %s:%s", src, d.bucket, dst)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(dst),
		Body:   f,
	})

	info, _ := f.Stat()

	return info.Size(), err
}
