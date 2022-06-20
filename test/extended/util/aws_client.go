package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// AwsClient struct
type AwsClient struct {
	svc *ec2.EC2
}

// InitAwsSession init session
func InitAwsSession() *AwsClient {
	mySession := session.Must(session.NewSession())
	aClient := &AwsClient{
		svc: ec2.New(mySession, aws.NewConfig()),
	}

	return aClient
}

// GetAwsInstanceID Get int svc instance ID
func (a *AwsClient) GetAwsInstanceID(instanceName string) (string, error) {
	filters := []*ec2.Filter{
		{
			Name: aws.String("tag:Name"),
			Values: []*string{
				aws.String(instanceName),
			},
		},
	}
	input := ec2.DescribeInstancesInput{Filters: filters}
	instanceInfo, err := a.svc.DescribeInstances(&input)

	if err != nil {
		return "", err
	}

	if len(instanceInfo.Reservations) < 1 {
		return "", fmt.Errorf("No instance found in current cluster with name %s", instanceName)
	}

	instanceID := instanceInfo.Reservations[0].Instances[0].InstanceId
	e2e.Logf("The %s instance id is %s .", instanceName, *instanceID)
	return *instanceID, err
}

// GetAwsIntIPs get aws int ip
func (a *AwsClient) GetAwsIntIPs(instanceID string) (map[string]string, error) {
	filters := []*ec2.Filter{
		{
			Name: aws.String("instance-id"),
			Values: []*string{
				aws.String(instanceID),
			},
		},
	}
	input := ec2.DescribeInstancesInput{Filters: filters}
	instanceInfo, err := a.svc.DescribeInstances(&input)
	if err != nil {
		return nil, err
	}

	if len(instanceInfo.Reservations) < 1 {
		return nil, fmt.Errorf("No instance found in current cluster with ID %s", instanceID)
	}

	privateIP := instanceInfo.Reservations[0].Instances[0].PrivateIpAddress
	publicIP := instanceInfo.Reservations[0].Instances[0].PublicIpAddress
	ips := make(map[string]string, 3)

	if publicIP == nil && privateIP == nil {
		e2e.Logf("There is no ips for this instance %s", instanceID)
		return nil, fmt.Errorf("There is no ips for this instance %s", instanceID)
	}

	if publicIP != nil {
		ips["publicIP"] = *publicIP
		e2e.Logf("The instance's public ip is %s", *publicIP)
	}

	if privateIP != nil {
		ips["privateIP"] = *privateIP
		e2e.Logf("The instance's private ip is %s", *privateIP)
	}

	return ips, nil
}

// UpdateAwsIntSecurityRule update int security rule
func (a *AwsClient) UpdateAwsIntSecurityRule(instanceID string, dstPort int64) error {
	filters := []*ec2.Filter{
		{
			Name: aws.String("instance-id"),
			Values: []*string{
				aws.String(instanceID),
			},
		},
	}
	input := ec2.DescribeInstancesInput{Filters: filters}
	instanceInfo, err := a.svc.DescribeInstances(&input)
	if err != nil {
		return err
	}

	if len(instanceInfo.Reservations) < 1 {
		return fmt.Errorf("No such instance ID in current cluster %s", instanceID)
	}

	securityGroupID := instanceInfo.Reservations[0].Instances[0].SecurityGroups[0].GroupId

	e2e.Logf("The instance's %s,security group id is %s .", instanceID, *securityGroupID)

	//Check if destination port is opned
	req := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{aws.String(*securityGroupID)},
	}
	resp, err := a.svc.DescribeSecurityGroups(req)
	if err != nil {
		return err
	}

	if strings.Contains(resp.GoString(), "ToPort: "+strconv.FormatInt(dstPort, 10)) {
		e2e.Logf("The destination port %v was opened in security group %s .", dstPort, *securityGroupID)
		return nil
	}

	//Update ingress secure rule to allow destination port
	_, err = a.svc.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(*securityGroupID),
		IpPermissions: []*ec2.IpPermission{
			(&ec2.IpPermission{}).
				SetIpProtocol("tcp").
				SetFromPort(dstPort).
				SetToPort(dstPort).
				SetIpRanges([]*ec2.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				}),
		},
	})

	if err != nil {
		e2e.Logf("Unable to set security group %s, ingress, %v", *securityGroupID, err)
		return err
	}

	e2e.Logf("Successfully update destination port %v to security group %s ingress rule.", dstPort, *securityGroupID)

	return nil
}

// GetAwsInstanceIDFromHostname Get instance ID from hostname
func (a *AwsClient) GetAwsInstanceIDFromHostname(hostname string) (string, error) {
	filters := []*ec2.Filter{
		{
			Name: aws.String("private-dns-name"),
			Values: []*string{
				aws.String(hostname),
			},
		},
	}
	input := ec2.DescribeInstancesInput{Filters: filters}
	instanceInfo, err := a.svc.DescribeInstances(&input)

	if err != nil {
		return "", err
	}

	if len(instanceInfo.Reservations) < 1 {
		return "", fmt.Errorf("No instance found in current cluster with name %s", hostname)
	}

	instanceID := instanceInfo.Reservations[0].Instances[0].InstanceId
	e2e.Logf("The %s instance id is %s .", hostname, *instanceID)
	return *instanceID, err
}

// StartInstance Start an instance
func (a *AwsClient) StartInstance(instanceID string) error {
	if instanceID == "" {
		e2e.Logf("You must supply an instance ID (-i INSTANCE-ID")
		return fmt.Errorf("You must supply an instance ID (-i INSTANCE-ID")
	}
	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			&instanceID,
		},
	}
	result, err := a.svc.StartInstances(input)
	e2e.Logf("%v", result.StartingInstances)
	return err
}

// StopInstance Stop an instance
func (a *AwsClient) StopInstance(instanceID string) error {
	if instanceID == "" {
		e2e.Logf("You must supply an instance ID (-i INSTANCE-ID")
		return fmt.Errorf("You must supply an instance ID (-i INSTANCE-ID")
	}
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{
			&instanceID,
		},
	}
	result, err := a.svc.StopInstances(input)
	e2e.Logf("%v", result.StoppingInstances)
	return err
}

// GetAwsInstanceState gives the instance state
func (a *AwsClient) GetAwsInstanceState(instanceID string) (string, error) {
	filters := []*ec2.Filter{
		{
			Name: aws.String("instance-id"),
			Values: []*string{
				aws.String(instanceID),
			},
		},
	}
	input := ec2.DescribeInstancesInput{Filters: filters}
	instanceInfo, err := a.svc.DescribeInstances(&input)
	if err != nil {
		return "", err
	}

	if len(instanceInfo.Reservations) < 1 {
		return "", fmt.Errorf("No instance found in current cluster with ID %s", instanceID)
	}

	instanceState := instanceInfo.Reservations[0].Instances[0].State.Name
	return *instanceState, err
}
