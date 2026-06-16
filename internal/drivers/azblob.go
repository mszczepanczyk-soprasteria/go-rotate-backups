package drivers

// This driver is mostly AI generated, I made some changes to ListDirs because
// Claude got quite confused there, but otherwise I just reviewed it and it
// looks alright. Initial tests were successful.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/sirupsen/logrus"
)

func init() {
	AddDriver("azblob", &AzureBlobDriver{})
}

// AzureBlobDriver stores backups in an Azure Blob Storage container.
type AzureBlobDriver struct {
	BaseDriver

	container string
	client    *azblob.Client
}

// getClient builds an azblob client based on the available environment.
// Authentication is resolved in the following order of precedence:
//   - AZURE_STORAGE_CONNECTION_STRING (connection string)
//   - AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY (shared key)
//   - AZURE_STORAGE_ACCOUNT + DefaultAzureCredential
//     (managed identity, workload identity, env service principal, Azure CLI, ...)
func (d *AzureBlobDriver) getClient() (*azblob.Client, error) {
	if connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING"); connStr != "" {
		logrus.Debug("Authenticating to Azure using a connection string")
		return azblob.NewClientFromConnectionString(connStr, nil)
	}

	account := os.Getenv("AZURE_STORAGE_ACCOUNT")
	if account == "" {
		return nil, fmt.Errorf("you need to set 'AZURE_STORAGE_ACCOUNT' or 'AZURE_STORAGE_CONNECTION_STRING'")
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", account)

	if key := os.Getenv("AZURE_STORAGE_KEY"); key != "" {
		logrus.Debug("Authenticating to Azure using a shared key")
		cred, err := azblob.NewSharedKeyCredential(account, key)
		if err != nil {
			return nil, err
		}
		return azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	}

	logrus.Debug("Authenticating to Azure using DefaultAzureCredential")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return azblob.NewClient(serviceURL, cred, nil)
}

func (d *AzureBlobDriver) Init() error {
	d.container = os.Getenv("GRB_AZURE_CONTAINER")
	if d.container == "" {
		return fmt.Errorf("you need to set 'GRB_AZURE_CONTAINER' for a target container")
	}

	client, err := d.getClient()
	if err != nil {
		return err
	}
	d.client = client

	// Verify connectivity and credentials with a lightweight list request.
	maxResults := int32(1)
	pager := d.client.NewListBlobsFlatPager(d.container, &azblob.ListBlobsFlatOptions{
		MaxResults: &maxResults,
	})
	if _, err := pager.NextPage(context.Background()); err != nil {
		return err
	}

	logrus.Debugf("Connected to Azure Blob container %s", d.container)
	return nil
}

func (d *AzureBlobDriver) ListDirs(path string) ([]string, error) {
	res := []string{}

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

  containerClient := d.client.ServiceClient().NewContainerClient(d.container)
	
	// Use a hierarchical listing with "/" as the delimiter so the API returns
	// only the immediate sub-directories (BlobPrefixes) instead of every blob.
	pager := containerClient.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{
		Prefix: &path,
	})

	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return res, err
		}
		for _, prefix := range page.Segment.BlobPrefixes {
			if prefix.Name == nil {
				continue
			}
			// Trim the search prefix and trailing slash to get the dir name.
			name := strings.TrimSuffix(strings.TrimPrefix(*prefix.Name, path), "/")
			res = append(res, name)
		}
	}

	logrus.Tracef("Listing dirs %s:%s -> %v", d.container, path, res)

	return res, nil
}

func (d *AzureBlobDriver) listRaw(path string) ([]string, error) {
	res := []string{}

	pager := d.client.NewListBlobsFlatPager(d.container, &azblob.ListBlobsFlatOptions{
		Prefix: &path,
	})

	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return res, err
		}
		for _, blob := range page.Segment.BlobItems {
			res = append(res, *blob.Name)
		}
	}

	logrus.Tracef("Listing %s:%s -> %v", d.container, path, res)

	return res, nil
}

func (d *AzureBlobDriver) Mkdir(path string) error {
	// Not needed in Azure Blob Storage
	return nil
}

func (d *AzureBlobDriver) Delete(src string) error {
	items, err := d.listRaw(src)
	if err != nil {
		return err
	}

	for _, item := range items {
		logrus.Tracef("Deleting %s:%s", d.container, item)
		if _, err := d.client.DeleteBlob(context.Background(), d.container, item, nil); err != nil {
			return err
		}
	}

	return nil
}

func (d *AzureBlobDriver) Copy(src, dst string) (int64, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %q, %v", src, err)
	}
	defer f.Close()

	logrus.Tracef("Uploading %s to %s:%s", src, d.container, dst)
	if _, err := d.client.UploadFile(context.Background(), d.container, dst, f, nil); err != nil {
		return 0, err
	}

	info, _ := f.Stat()

	return info.Size(), nil
}

