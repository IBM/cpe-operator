/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package common

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
)

type COSObject struct {
	RawBucketName     string
	APIKey            string
	ServiceInstanceID string
	AuthEndpoint      string
	ServiceEndpoint   string
}

func (c *COSObject) InitValue() {
	c.APIKey = os.Getenv("CPE_COS_LOG_APIKEY")
	c.ServiceInstanceID = os.Getenv("CPE_COS_LOG_SERVICE_ID")
	c.AuthEndpoint = os.Getenv("CPE_COS_LOG_AUTH_ENDPOINT")
	c.ServiceEndpoint = os.Getenv("CPE_COS_LOG_SERVICE_ENDPOINT")
	c.RawBucketName = os.Getenv("CPE_COS_LOG_RAW_BUCKET")
}

func NewCOS() *COSObject {
	cos := COSObject{}
	cos.InitValue()
	return &cos
}

func PutLog(c *COSObject, keyName string, podLogs []byte) error {
	if c.ServiceEndpoint == "" {
		return fmt.Errorf("No ServiceEndpoint set")
	}

	reader := bytes.NewReader(podLogs)
	ctx := context.Background()

	bucketName := c.RawBucketName

	conf := aws.NewConfig().
		WithEndpoint(c.ServiceEndpoint).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(),
			c.AuthEndpoint, c.APIKey, c.ServiceInstanceID)).
		WithS3ForcePathStyle(true)

	sess := session.Must(session.NewSession())
	client := s3.New(sess, conf)

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	client.CreateBucket(input)

	_, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(keyName),
		Body:   reader,
	})
	return err
}

func GetLog(c *COSObject, keyName string) (b []byte, err error) {
	if c.ServiceEndpoint == "" {
		return nil, fmt.Errorf("No ServiceEndpoint set")
	}

	bucketName := c.RawBucketName

	conf := aws.NewConfig().
		WithEndpoint(c.ServiceEndpoint).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(),
			c.AuthEndpoint, c.APIKey, c.ServiceInstanceID)).
		WithS3ForcePathStyle(true)

	sess := session.Must(session.NewSession())
	client := s3.New(sess, conf)

	// users will need to create bucket, key (flat string name)
	input := s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(keyName),
	}

	// Call Function
	res, err := client.GetObject(&input)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(res.Body)

}
