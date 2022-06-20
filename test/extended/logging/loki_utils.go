package logging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"google.golang.org/api/iterator"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	apiPath         = "/api/logs/v1/"
	queryPath       = "/loki/api/v1/query"
	queryRangePath  = "/loki/api/v1/query_range"
	labelsPath      = "/loki/api/v1/labels"
	labelValuesPath = "/loki/api/v1/label/%s/values"
	seriesPath      = "/loki/api/v1/series"
	tailPath        = "/loki/api/v1/tail"
)

// awsCredential defines the aws credentials
type awsCredential struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

func getAWSClusterRegion(oc *exutil.CLI) (string, error) {
	region, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.aws.region}").Output()
	return region, err
}

func getAWSCredentialFromCluster(oc *exutil.CLI) awsCredential {
	region, err := getAWSClusterRegion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	accessKeyID, secureKey := getAWSKey(oc)
	cred := awsCredential{Region: region, AccessKeyID: string(accessKeyID), SecretAccessKey: string(secureKey)}
	return cred
}

// initialize an aws s3 client with aws credential
// TODO: add an option to initialize a new client with STS
func newAWSS3Client(oc *exutil.CLI) *s3.Client {
	cred := getAWSCredentialFromCluster(oc)
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cred.AccessKeyID, cred.SecretAccessKey, "")),
		config.WithRegion(cred.Region))
	o.Expect(err).NotTo(o.HaveOccurred())
	return s3.NewFromConfig(cfg)
}

func createAWSS3Bucket(oc *exutil.CLI, client *s3.Client, bucketName string) error {
	region, err := getAWSClusterRegion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	// check if the bucket exists or not
	// if exists, clear all the objects in the bucket
	// if not, create the bucket
	exist := false
	buckets, err := client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, bu := range buckets.Buckets {
		if *bu.Name == bucketName {
			exist = true
			break
		}
	}
	// clear all the objects in the bucket
	if exist {
		return emptyAWSS3Bucket(client, bucketName)
	}
	_, err = client.CreateBucket(context.TODO(), &s3.CreateBucketInput{Bucket: &bucketName, CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(region)}})
	return err

}

func deleteAWSS3Bucket(client *s3.Client, bucketName string) {
	// empty bucket
	emptyAWSS3Bucket(client, bucketName)
	// delete bucket
	_, err := client.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{Bucket: &bucketName})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func emptyAWSS3Bucket(client *s3.Client, bucketName string) error {
	// list objects in the bucket
	objects, err := client.ListObjects(context.TODO(), &s3.ListObjectsInput{Bucket: &bucketName})
	// remove objects in the bucket
	if len(objects.Contents) != 0 {
		newObjects := []types.ObjectIdentifier{}
		for _, object := range objects.Contents {
			newObjects = append(newObjects, types.ObjectIdentifier{Key: object.Key})
		}
		_, err = client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{Bucket: &bucketName, Delete: &types.Delete{Quiet: true, Objects: newObjects}})
		return err
	}
	return err
}

// createSecretForAWSS3Bucket creates a secret for Loki to connect to s3 bucket
func createSecretForAWSS3Bucket(oc *exutil.CLI, bucketName, secretName, ns string) error {
	if len(secretName) == 0 {
		return fmt.Errorf("secret name shouldn't be empty")
	}
	cred := getAWSCredentialFromCluster(oc)
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.RemoveAll(dirname)
	f1, err1 := os.Create(dirname + "/aws_access_key_id")
	o.Expect(err1).NotTo(o.HaveOccurred())
	defer f1.Close()
	_, err = f1.WriteString(cred.AccessKeyID)
	o.Expect(err).NotTo(o.HaveOccurred())
	f2, err2 := os.Create(dirname + "/aws_secret_access_key")
	o.Expect(err2).NotTo(o.HaveOccurred())
	defer f2.Close()
	_, err = f2.WriteString(cred.SecretAccessKey)
	o.Expect(err).NotTo(o.HaveOccurred())

	endpoint := "https://s3." + cred.Region + ".amazonaws.com"
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", secretName, "--from-file=access_key_id="+dirname+"/aws_access_key_id", "--from-file=access_key_secret="+dirname+"/aws_secret_access_key", "--from-literal=region="+cred.Region, "--from-literal=bucketnames="+bucketName, "--from-literal=endpoint="+endpoint, "-n", ns).Execute()
}

func getGCPProjectID(oc *exutil.CLI) string {
	projectID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructures.config.openshift.io", "cluster", "-ojsonpath={.status.platformStatus.gcp.projectID}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return projectID
}

func createGCSBucket(projectID, bucketName string) error {
	ctx := context.Background()
	// initialize the GCS client, the credentials are got from the env var GOOGLE_APPLICATION_CREDENTIALS
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	// check if the bucket exists or not
	// if exists, clear all the objects in the bucket
	// if not, create the bucket
	exist := false
	buckets, err := listGCSBuckets(*client, projectID)
	if err != nil {
		return err
	}
	for _, bu := range buckets {
		if bu == bucketName {
			exist = true
			break
		}
	}
	if exist {
		emptyGCSBucket(*client, bucketName)
	}

	bucket := client.Bucket(bucketName)
	if err := bucket.Create(ctx, projectID, &storage.BucketAttrs{}); err != nil {
		return fmt.Errorf("Bucket(%q).Create: %v", bucketName, err)
	}
	fmt.Printf("Created bucket %v\n", bucketName)
	return nil
}

// listGCSBuckets gets all the bucket names under the projectID
func listGCSBuckets(client storage.Client, projectID string) ([]string, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	var buckets []string
	it := client.Buckets(ctx, projectID)
	for {
		battrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, battrs.Name)
	}
	return buckets, nil
}

// listObjestsInGCSBucket gets all the objects in a bucket
func listObjestsInGCSBucket(client storage.Client, bucket string) ([]string, error) {
	files := []string{}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	it := client.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return files, fmt.Errorf("Bucket(%q).Objects: %v", bucket, err)
		}
		files = append(files, attrs.Name)
	}
	return files, nil
}

// deleteFilesInGCSBucket removes all the objexts in the bucket
func deleteFilesInGCSBucket(client storage.Client, object, bucket string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	o := client.Bucket(bucket).Object(object)

	// Optional: set a generation-match precondition to avoid potential race
	// conditions and data corruptions. The request to upload is aborted if the
	// object's generation number does not match your precondition.
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("object.Attrs: %v", err)
	}
	o = o.If(storage.Conditions{GenerationMatch: attrs.Generation})

	if err := o.Delete(ctx); err != nil {
		return fmt.Errorf("Object(%q).Delete: %v", object, err)
	}
	return nil
}

func emptyGCSBucket(client storage.Client, bucket string) {
	objects, err := listObjestsInGCSBucket(client, bucket)
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(objects) > 0 {
		for _, object := range objects {
			err = deleteFilesInGCSBucket(client, object, bucket)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}
}

func deleteGCSBucket(bucketName string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	// remove objects
	emptyGCSBucket(*client, bucketName)
	bucket := client.Bucket(bucketName)
	if err := bucket.Delete(ctx); err != nil {
		return fmt.Errorf("Bucket(%q).Delete: %v", bucketName, err)
	}
	fmt.Printf("Bucket %v deleted\n", bucketName)
	return nil
}

// creates a secret for Loki to connect to gcs bucket
func createSecretForGCSBucket(oc *exutil.CLI, bucketName, secretName, ns string) error {
	if len(secretName) == 0 {
		return fmt.Errorf("secret name shouldn't be empty")
	}
	dirname := "/tmp/" + oc.Namespace() + "-creds"
	defer os.RemoveAll(dirname)
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/gcp-credentials", "-n", "kube-system", "--confirm", "--to="+dirname).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", secretName, "-n", ns, "--from-literal=bucketname="+bucketName, "--from-file=key.json="+dirname+"/service_account.json").Execute()
}

// get azure storage account from image registry
// TODO: create a storage account and use that accout to manage azure container
func getAzureStorageAccount(oc *exutil.CLI) (string, string) {
	var accountName string
	imageRegistry, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-image-registry").Get("image-registry", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, container := range imageRegistry.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == "REGISTRY_STORAGE_AZURE_ACCOUNTNAME" {
				accountName = env.Value
				break
			}
		}
	}

	dirname := "/tmp/" + oc.Namespace() + "-creds"
	defer os.RemoveAll(dirname)
	err = os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/image-registry-private-configuration", "-n", "openshift-image-registry", "--confirm", "--to="+dirname).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	accountKey, err := os.ReadFile(dirname + "/REGISTRY_STORAGE_AZURE_ACCOUNTKEY")
	o.Expect(err).NotTo(o.HaveOccurred())
	return accountName, string(accountKey)
}

// initialize a new azure blob container client
func newAzureContainerClient(oc *exutil.CLI, name string) azblob.ContainerURL {
	accountName, accountKey := getAzureStorageAccount(oc)
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	o.Expect(err).NotTo(o.HaveOccurred())
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL := azblob.NewServiceURL(*u, p)
	return serviceURL.NewContainerURL(name)
}

func createAzureStorageBlobContainer(container azblob.ContainerURL) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	// check if the container exists or not
	// if exists, then remove the blobs in the container, if not, create the container
	_, err := container.GetProperties(ctx, azblob.LeaseAccessConditions{})
	message := fmt.Sprintf("%v", err)
	if strings.Contains(message, "ContainerNotFound") {
		_, err = container.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
		return err
	}
	emptyAzureBlobContainer(container)
	return nil
}

func deleteAzureStorageBlobContainer(container azblob.ContainerURL) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	_, err := container.Delete(ctx, azblob.ContainerAccessConditions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func listBlobsInAzureContainer(container azblob.ContainerURL) []string {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	var blobNames []string
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := container.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			blobNames = append(blobNames, blobInfo.Name)
		}
	}
	return blobNames
}

func deleteAzureBlob(container azblob.ContainerURL, blobName string) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	blobURL := container.NewBlockBlobURL(blobName)
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func emptyAzureBlobContainer(container azblob.ContainerURL) {
	blobNames := listBlobsInAzureContainer(container)
	if len(blobNames) > 0 {
		for _, blob := range blobNames {
			deleteAzureBlob(container, blob)
		}
	}
}

// creates a secret for Loki to connect to azure container
func createSecretForAzureContainer(oc *exutil.CLI, bucketName, secretName, ns string) error {
	environment := "AzureGlobal"
	accountName, accountKey := getAzureStorageAccount(oc)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("secret", "generic", "-n", ns, secretName, "--from-literal=environment="+environment, "--from-literal=container="+bucketName, "--from-literal=account_name="+accountName, "--from-literal=account_key="+accountKey).Execute()
	return err
}

// return the storage type per different platform
func getStorageType(oc *exutil.CLI) string {
	platform := exutil.CheckPlatform(oc)
	switch platform {
	case "aws":
		{
			return "s3"
		}
	case "gcp":
		{
			return "gcs"
		}
	case "azure":
		{
			return "azure"
		}
	case "openstack":
		{
			return "swift"
		}
	default:
		{
			return ""
		}
	}
}

// lokiStack contains the configurations of loki stack
type lokiStack struct {
	name              string // lokiStack name
	namespace         string // lokiStack namespace
	tSize             string // size
	storageType       string // the backend storage type, currently support s3, gcs and azure
	storageSecret     string // the secret name for loki to use to connect to backend storage
	replicationFactor string // replicationFactor
	storageClass      string // storage class name
	bucketName        string // the butcket or the container name where loki stores it's data in
	template          string // the file used to create the loki stack
}

// prepareResourcesForLokiStack creates buckets/containers in backend storage provider, and creates the secret for Loki to use
func (l lokiStack) prepareResourcesForLokiStack(oc *exutil.CLI) error {
	var err error
	if len(l.bucketName) == 0 {
		return fmt.Errorf("the bucketName should not be empty")
	}
	switch l.storageType {
	case "s3":
		{
			client := newAWSS3Client(oc)
			err1 := createAWSS3Bucket(oc, client, l.bucketName)
			o.Expect(err1).NotTo(o.HaveOccurred())
			err = createSecretForAWSS3Bucket(oc, l.bucketName, l.storageSecret, l.namespace)
		}
	case "azure":
		{
			client := newAzureContainerClient(oc, l.bucketName)
			err1 := createAzureStorageBlobContainer(client)
			o.Expect(err1).NotTo(o.HaveOccurred())
			err = createSecretForAzureContainer(oc, l.bucketName, l.storageSecret, l.namespace)
		}
	case "gcs":
		{
			err1 := createGCSBucket(getGCPProjectID(oc), l.bucketName)
			o.Expect(err1).NotTo(o.HaveOccurred())
			err = createSecretForGCSBucket(oc, l.bucketName, l.storageSecret, l.namespace)
		}
	case "swift":
		{
			return fmt.Errorf("deploy loki with %s is under development", l.storageType)
		}
	default:
		{
			return fmt.Errorf("unsupported storage type %s", l.storageType)
		}
	}
	return err
}

// deployLokiStack creates the lokiStack CR with basic settings: name, namespace, replicationFactor, size, storage.secret.name, storage.secret.type, storageClassName
// optionalParameters is designed for adding parameters to deploy lokiStack with different tenants
func (l lokiStack) deployLokiStack(oc *exutil.CLI, optionalParameters ...string) error {
	parameters := []string{"-f", l.template, "-n", l.namespace, "-p", "NAME=" + l.name, "NAMESPACE=" + l.namespace, "REPLICAS_FACTOR=" + l.replicationFactor, "SIZE=" + l.tSize, "SECRET_NAME=" + l.storageSecret, "STORAGE_TYPE=" + l.storageType, "STORAGE_CLASS=" + l.storageClass}
	if len(optionalParameters) != 0 {
		parameters = append(parameters, optionalParameters...)
	}
	file, err := processTemplate(oc, parameters...)
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("Can not process %v", parameters))
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", file, "-n", l.namespace).Execute()
	ls := resource{"lokistack", l.name, l.namespace}
	ls.WaitForResourceToAppear(oc)
	return err
}

func (l lokiStack) waitForLokiStackToBeReady(oc *exutil.CLI) {
	for _, deploy := range []string{l.name + "-distributor", l.name + "-gateway", l.name + "-querier", l.name + "-query-frontend"} {
		WaitForDeploymentPodsToBeReady(oc, l.namespace, deploy)
	}
	for _, ss := range []string{l.name + "-compactor", l.name + "-index-gateway", l.name + "-ingester"} {
		waitForStatefulsetReady(oc, l.namespace, ss)
	}
}

func (l lokiStack) removeLokiStack(oc *exutil.CLI) {
	resource{"lokistack", l.name, l.namespace}.clear(oc)
	resource{"secret", l.storageSecret, l.namespace}.clear(oc)
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("pvc", "-n", l.namespace, "-l", "app.kubernetes.io/instance="+l.name).Execute()
	switch l.storageType {
	case "s3":
		{
			client := newAWSS3Client(oc)
			deleteAWSS3Bucket(client, l.bucketName)
		}
	case "azure":
		{
			client := newAzureContainerClient(oc, l.bucketName)
			emptyAzureBlobContainer(client)
			deleteAzureStorageBlobContainer(client)
		}
	case "gcs":
		{
			err := deleteGCSBucket(l.bucketName)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	case "swift":
		{
			e2e.Logf("Deploy loki with %s is under development", l.storageType)
		}
	}
}

func grantLokiPermissionsToSA(oc *exutil.CLI, rbacName, sa, ns string) {
	rbac := exutil.FixturePath("testdata", "logging", "lokistack", "loki-rbac.yaml")
	file, err := processTemplate(oc, "-f", rbac, "-p", "NAME="+rbacName, "-p", "SA="+sa, "NAMESPACE="+ns)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", file).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func removeLokiStackPermissionFromSA(oc *exutil.CLI, rbacName string) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrole/"+rbacName, "clusterrolebinding/"+rbacName).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// TODO: add an option to provide TLS config
type lokiClient struct {
	username        string //Username for HTTP basic auth.
	password        string //Password for HTTP basic auth
	address         string //Server address.
	orgID           string //adds X-Scope-OrgID to API requests for representing tenant ID. Useful for requesting tenant data when bypassing an auth gateway.
	bearerToken     string //adds the Authorization header to API requests for authentication purposes.
	bearerTokenFile string //adds the Authorization header to API requests for authentication purposes.
	retries         int    //How many times to retry each query when getting an error response from Loki.
	queryTags       string //adds X-Query-Tags header to API requests.
	quiet           bool   //Suppress query metadata.
}

func (c *lokiClient) getHTTPRequestHeader() (http.Header, error) {
	h := make(http.Header)
	if c.username != "" && c.password != "" {
		h.Set(
			"Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password)),
		)
	}
	h.Set("User-Agent", "loki-logcli")

	if c.orgID != "" {
		h.Set("X-Scope-OrgID", c.orgID)
	}

	if c.queryTags != "" {
		h.Set("X-Query-Tags", c.queryTags)
	}

	if (c.username != "" || c.password != "") && (len(c.bearerToken) > 0 || len(c.bearerTokenFile) > 0) {
		return nil, fmt.Errorf("at most one of HTTP basic auth (username/password), bearer-token & bearer-token-file is allowed to be configured")
	}

	if len(c.bearerToken) > 0 && len(c.bearerTokenFile) > 0 {
		return nil, fmt.Errorf("at most one of the options bearer-token & bearer-token-file is allowed to be configured")
	}

	if c.bearerToken != "" {
		h.Set("Authorization", "Bearer "+c.bearerToken)
	}

	if c.bearerTokenFile != "" {
		b, err := ioutil.ReadFile(c.bearerTokenFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read authorization credentials file %s: %s", c.bearerTokenFile, err)
		}
		bearerToken := strings.TrimSpace(string(b))
		h.Set("Authorization", "Bearer "+bearerToken)
	}
	return h, nil
}

func (c *lokiClient) doRequest(path, query string, quiet bool, out interface{}) error {
	us, err := buildURL(c.address, path, query)
	if err != nil {
		return err
	}
	if !quiet {
		e2e.Logf(us)
	}

	req, err := http.NewRequest("GET", us, nil)
	if err != nil {
		return err
	}

	h, err := c.getHTTPRequestHeader()
	if err != nil {
		return err
	}
	req.Header = h

	var tr *http.Transport
	if os.Getenv("http_proxy") != "" || os.Getenv("https_proxy") != "" {
		var proxy string
		if os.Getenv("http_proxy") != "" {
			proxy = os.Getenv("http_proxy")
		} else {
			proxy = os.Getenv("https_proxy")
		}
		proxyURL, err := url.Parse(proxy)
		o.Expect(err).NotTo(o.HaveOccurred())
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyURL),
		}
	} else {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client := &http.Client{Transport: tr}

	var resp *http.Response
	attempts := c.retries + 1
	success := false

	for attempts > 0 {
		attempts--

		resp, err = client.Do(req)
		if err != nil {
			e2e.Logf("error sending request %v", err)
			continue
		}
		if resp.StatusCode/100 != 2 {
			buf, _ := ioutil.ReadAll(resp.Body) // nolint
			e2e.Logf("Error response from server: %s (%v) attempts remaining: %d", string(buf), err, attempts)
			if err := resp.Body.Close(); err != nil {
				e2e.Logf("error closing body", err)
			}
			continue
		}
		success = true
		break
	}
	if !success {
		return fmt.Errorf("run out of attempts while querying the server")
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			e2e.Logf("error closing body", err)
		}
	}()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *lokiClient) doQuery(path string, query string, quiet bool) (*lokiQueryResponse, error) {
	var err error
	var r lokiQueryResponse

	if err = c.doRequest(path, query, quiet, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

/*
//query uses the /api/v1/query endpoint to execute an instant query
func (c *lokiClient) query(logType string, queryStr string, limit int, forward bool, time time.Time) (*lokiQueryResponse, error) {
	direction := func() string {
		if forward {
			return "FORWARD"
		}
		return "BACKWARD"
	}
	qsb := newQueryStringBuilder()
	qsb.setString("query", queryStr)
	qsb.setInt("limit", int64(limit))
	qsb.setInt("time", time.UnixNano())
	qsb.setString("direction", direction())
	logPath := apiPath + logType + queryPath
	return c.doQuery(logPath, qsb.encode(), c.quiet)
}
*/

//queryRange uses the /api/v1/query_range endpoint to execute a range query
//logType: application, infrastructure, audit
//queryStr: string to filter logs, for example: "{kubernetes_namespace_name="test"}"
//limit: max log count
//start: Start looking for logs at this absolute time(inclusive), e.g.: time.Now().Add(time.Duration(-1)*time.Hour) means 1 hour ago
//end: Stop looking for logs at this absolute time (exclusive)
//forward: true means scan forwards through logs, false means scan backwards through logs
func (c *lokiClient) queryRange(logType string, queryStr string, limit int, start, end time.Time, forward bool) (*lokiQueryResponse, error) {
	direction := func() string {
		if forward {
			return "FORWARD"
		}
		return "BACKWARD"
	}
	params := newQueryStringBuilder()
	params.setString("query", queryStr)
	params.setInt32("limit", limit)
	params.setInt("start", start.UnixNano())
	params.setInt("end", end.UnixNano())
	params.setString("direction", direction())
	logPath := apiPath + logType + queryRangePath
	return c.doQuery(logPath, params.encode(), c.quiet)
}

// buildURL concats a url `http://foo/bar` with a path `/buzz`.
func buildURL(u, p, q string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	url.Path = path.Join(url.Path, p)
	url.RawQuery = q
	return url.String(), nil
}

type queryStringBuilder struct {
	values url.Values
}

func newQueryStringBuilder() *queryStringBuilder {
	return &queryStringBuilder{
		values: url.Values{},
	}
}

func (b *queryStringBuilder) setString(name, value string) {
	b.values.Set(name, value)
}

func (b *queryStringBuilder) setInt(name string, value int64) {
	b.setString(name, strconv.FormatInt(value, 10))
}

func (b *queryStringBuilder) setInt32(name string, value int) {
	b.setString(name, strconv.Itoa(value))
}

/*
func (b *queryStringBuilder) setStringArray(name string, values []string) {
	for _, v := range values {
		b.values.Add(name, v)
	}
}
func (b *queryStringBuilder) setFloat32(name string, value float32) {
	b.setString(name, strconv.FormatFloat(float64(value), 'f', -1, 32))
}
func (b *queryStringBuilder) setFloat(name string, value float64) {
	b.setString(name, strconv.FormatFloat(value, 'f', -1, 64))
}
*/

// encode returns the URL-encoded query string based on key-value
// parameters added to the builder calling Set functions.
func (b *queryStringBuilder) encode() string {
	return b.values.Encode()
}

// getSchedulableLinuxWorkerNodes returns a group of nodes that match the requirements:
// os: linux, role: worker, status: ready, schedulable
func getSchedulableLinuxWorkerNodes(oc *exutil.CLI) ([]v1.Node, error) {
	var nodes, workers []v1.Node
	linuxNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "kubernetes.io/os=linux"})
	// get schedulable linux worker nodes
	for _, node := range linuxNodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok && !node.Spec.Unschedulable {
			workers = append(workers, node)
		}
	}
	// get ready nodes
	for _, worker := range workers {
		for _, con := range worker.Status.Conditions {
			if con.Type == "Ready" && con.Status == "True" {
				nodes = append(nodes, worker)
				break
			}
		}
	}
	return nodes, err
}

// getPodsNodesMap returns all the running pods in each node
func getPodsNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string][]v1.Pod {
	podsMap := make(map[string][]v1.Pod)
	projects, err := oc.AdminKubeClient().CoreV1().Namespaces().List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// get pod list in each node
	for _, project := range projects.Items {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(project.Name).List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, pod := range pods.Items {
			if pod.Status.Phase != "Failed" && pod.Status.Phase != "Succeeded" {
				podsMap[pod.Spec.NodeName] = append(podsMap[pod.Spec.NodeName], pod)
			}
		}
	}

	var nodeNames []string
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	// if the key is not in nodes list, remove the element from the map
	for podmap := range podsMap {
		if !contain(nodeNames, podmap) {
			delete(podsMap, podmap)
		}
	}
	return podsMap
}

type resList struct {
	cpu    int64
	memory int64
}

// getRequestedResourcesNodesMap returns the requested CPU and Memory in each node
func getRequestedResourcesNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string]resList {
	rmap := make(map[string]resList)
	podsMap := getPodsNodesMap(oc, nodes)
	for nodeName := range podsMap {
		var totalRequestedCPU, totalRequestedMemory int64
		for _, pod := range podsMap[nodeName] {
			for _, container := range pod.Spec.Containers {
				totalRequestedCPU += container.Resources.Requests.Cpu().MilliValue()
				totalRequestedMemory += container.Resources.Requests.Memory().MilliValue()
			}
		}
		rmap[nodeName] = resList{totalRequestedCPU, totalRequestedMemory}
	}
	return rmap
}

// getAllocatableResourcesNodesMap returns the allocatable CPU and Memory in each node
func getAllocatableResourcesNodesMap(nodes []v1.Node) map[string]resList {
	rmap := make(map[string]resList)
	for _, node := range nodes {
		rmap[node.Name] = resList{node.Status.Allocatable.Cpu().MilliValue(), node.Status.Allocatable.Memory().MilliValue()}
	}
	return rmap
}

// getRemainingResourcesNodesMap returns the remaning CPU and Memory in each node
func getRemainingResourcesNodesMap(oc *exutil.CLI, nodes []v1.Node) map[string]resList {
	rmap := make(map[string]resList)
	requested := getRequestedResourcesNodesMap(oc, nodes)
	allocatable := getAllocatableResourcesNodesMap(nodes)

	for _, node := range nodes {
		rmap[node.Name] = resList{allocatable[node.Name].cpu - requested[node.Name].cpu, allocatable[node.Name].memory - requested[node.Name].memory}
	}
	return rmap
}

// compareClusterResources compares the remaning resource with the requested resource provide by user
func compareClusterResources(oc *exutil.CLI, cpu, memory string) bool {
	nodes, err := getSchedulableLinuxWorkerNodes(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	var remainingCPU, remainingMemory int64
	re := getRemainingResourcesNodesMap(oc, nodes)
	for _, node := range nodes {
		remainingCPU += re[node.Name].cpu
		remainingMemory += re[node.Name].memory
	}

	requiredCPU, _ := k8sresource.ParseQuantity(cpu)
	requiredMemory, _ := k8sresource.ParseQuantity(memory)
	e2e.Logf("the required cpu is: %d, and the required memory is: %d", requiredCPU.MilliValue(), requiredMemory.MilliValue())
	e2e.Logf("the remaining cpu is: %d, and the remaning memory is: %d", remainingCPU, remainingMemory)
	return remainingCPU > requiredCPU.MilliValue() && remainingMemory > requiredMemory.MilliValue()
}

// validateInfraAndResourcesForLoki checks cluster remaning resources and platform type
// supportedPlatforms the platform types which the case can be executed on
func validateInfraAndResourcesForLoki(oc *exutil.CLI, supportedPlatforms []string, reqMemory, reqCPU string) bool {
	currentPlatform := exutil.CheckPlatform(oc)
	if currentPlatform == "aws" {
		// skip the case on aws sts clusters
		_, err := oc.AdminKubeClient().CoreV1().Secrets("kube-system").Get("aws-creds", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false
		}
	}
	return contain(supportedPlatforms, currentPlatform) && compareClusterResources(oc, reqCPU, reqMemory)
}
