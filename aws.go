package main

// TODO: If all ports are open, use public IP

import (
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const tmplAWS = `
{{ range $_, $instance := $.Instances -}}
Host {{ $instance.Domain }}
  HostName {{ $instance.IP }}
  {{ if $instance.ProxyJump -}}
  ProxyJump {{ $instance.ProxyJump }}
  {{ end }}
{{ end -}}

Host {{ $.Prefix }}*
  {{ if $.User -}}
  User {{ $.User }}
  {{ end -}}
  IdentityFile {{ $.IdentityFile }}
`

func getName(inst *ec2.Instance) string {

	// Try to find the domain tag. For instances where this doesn't
	// exist, fall back to the IP
	for _, tag := range inst.Tags {
		if *tag.Key == "Name" {
			return strings.Replace(*tag.Value, " ", "-", -1)
		}
	}

	if inst.PublicIpAddress != nil {
		return *inst.PublicIpAddress
	}

	return *inst.PrivateIpAddress
}

func getTag(inst *ec2.Instance, tagName string) string {
	for _, tag := range inst.Tags {
		if *tag.Key == tagName {
			return *tag.Value
		}
	}

	return ""
}

func parseFilter(filterStr string) *ec2.Filter {
	nameValueParts := strings.Split(filterStr, ",")
	if len(nameValueParts) != 2 {

	}

	nameParts := strings.Split(nameValueParts[0], "=")
	if len(nameParts) != 2 {
		fmt.Println("nameParts must equal to 2")
		os.Exit(1)
	}
	if nameParts[0] != "Name" {
		fmt.Println("nameParts[0] must be 'Name'")
		os.Exit(1)
	}

	valueParts := strings.Split(nameValueParts[1], "=")
	if len(valueParts) != 2 {
		fmt.Println("valueParts must equal to 2")
		os.Exit(1)
	}
	if valueParts[0] != "Values" {
		fmt.Println("valueParts[0] must be 'Values'")
		os.Exit(1)
	}

	filter := &ec2.Filter{
		Name:   aws.String(nameParts[1]),
		Values: []*string{aws.String(valueParts[1])},
	}
	return filter
	// 'Name=ip-permission.from-port,Values=22' 'Name=ip-permission.to-port,Values=22'
}

func findRouteTableForInstance(instance *ec2.Instance, routeTables *ec2.DescribeRouteTablesOutput) *ec2.RouteTable {
	var tablesForVpc []*ec2.RouteTable
	for _, table := range routeTables.RouteTables {
		if *instance.VpcId == *table.VpcId {
			tablesForVpc = append(tablesForVpc, table)
		}
	}

	var mainTable *ec2.RouteTable
	for _, table := range tablesForVpc {

		// Try to find an explicitly associated instance
		for _, assoc := range table.Associations {
			if assoc.SubnetId != nil && *assoc.SubnetId == *instance.SubnetId {
				return table
			}
			if *assoc.Main {
				mainTable = table
			}
		}
	}

	// Fall back to default route table
	return mainTable
}

func instanceIsPublic(instance *ec2.Instance, routeTables *ec2.DescribeRouteTablesOutput) bool {
	routeTable := findRouteTableForInstance(instance, routeTables)
	for _, route := range routeTable.Routes {
		if route.GatewayId != nil &&
			strings.HasPrefix(*route.GatewayId, "igw-") &&
			*route.DestinationCidrBlock == "0.0.0.0/0" {
			return true
		}
	}
	return false
}

func isInstanceInPublicSubnet(ec2Client *ec2.EC2, inst *ec2.Instance) bool {
	routeTableRes, err := ec2Client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("association.subnet-id"),
				Values: []*string{inst.SubnetId},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// uses default route table
	if len(routeTableRes.RouteTables) == 0 {
		routeTableRes, err = ec2Client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
			Filters: []*ec2.Filter{
				&ec2.Filter{
					Name:   aws.String("association.main"),
					Values: []*string{aws.String("true")},
				},
			},
		})
		if err != nil {
			panic(err)
		}
	}

	for _, routeTable := range routeTableRes.RouteTables {
		for _, r := range routeTable.Routes {
			if r.GatewayId != nil && strings.HasPrefix(*r.GatewayId, "igw") {
				return true
			}
		}
	}

	return false
}

func isPortOpen(ec2Client *ec2.EC2, inst *ec2.Instance) bool {
	securityGroupIds := []*string{}
	for _, group := range inst.SecurityGroups {
		securityGroupIds = append(securityGroupIds, group.GroupId)
	}
	securityGroupOutput, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: securityGroupIds,
	})
	if err != nil {
		panic(err)
	}

	for _, securityGroup := range securityGroupOutput.SecurityGroups {
		for _, permission := range securityGroup.IpPermissions {
			protocolOk := (*permission.IpProtocol == "tcp" || *permission.IpProtocol == "-1")
			publicIpRanges := usesPublicIpRanges(permission.IpRanges)

			if permission.FromPort == nil && permission.ToPort == nil &&
				publicIpRanges &&
				protocolOk {
				return true
			}

			if permission.FromPort != nil && *permission.FromPort <= 22 &&
				permission.ToPort != nil && *permission.ToPort >= 22 &&
				publicIpRanges &&
				protocolOk {
				return true

			}
		}
	}

	return false
}

func usesPublicIpRanges(ipRanges []*ec2.IpRange) bool {
	for _, r := range ipRanges {
		if *r.CidrIp == "0.0.0.0/0" {
			return true
		}
	}

	return false
}

func findJumpHost(instances []*ec2.Instance, jumphostTagName string) *ec2.Instance {
	jumphosts := []*ec2.Instance{}
	for _, instance := range instances {
		name := getName(instance)
		if strings.Contains(name, jumphostTagName) {
			jumphosts = append(jumphosts, instance)
		}
	}

	if len(jumphosts) != 1 {
		fmt.Println("Did not find exactly 1 jumphost, found", len(jumphosts))
		os.Exit(1)
	}

	return jumphosts[0]
}

func generateAWS(prefix string) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	ec2Client := ec2.New(sess)

	ec2Filters := []*ec2.Filter{
		&ec2.Filter{
			Name: aws.String("instance-state-name"),
			Values: []*string{
				aws.String("running"),
			},
		},
	}

	if len(filters) > 0 {
		ec2Filters = append(ec2Filters, parseFilter(filters))
	}

	describeInstancesOutput, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: ec2Filters,
	})
	checkErr(err)

	instances := []*ec2.Instance{}
	for _, r := range describeInstancesOutput.Reservations {
		for _, instance := range r.Instances {
			instances = append(instances, instance)
		}
	}

	templateData := struct {
		Prefix       string
		User         string
		IdentityFile string
		Instances    []map[string]string
	}{
		Prefix:       prefix,
		User:         sshUser,
		IdentityFile: identityFile,
		Instances:    []map[string]string{},
	}

	var jumphostHostname string
	if jumphost != "" {
		jumphostInstance := findJumpHost(instances, jumphost)
		jumphostName := getName(jumphostInstance)
		jumphostHostname = prefix + "-" + jumphostName
	}

	for _, instance := range instances {
		isInPublicSubnet := isInstanceInPublicSubnet(ec2Client, instance)
		isPortOpen := isPortOpen(ec2Client, instance)
		name := getName(instance)
		hostName := prefix + name

		var tmplData map[string]string

		if privateIP {
			tmplData = map[string]string{
				"Domain": hostName,
				"IP":     *instance.PrivateIpAddress,
			}
		} else if isInPublicSubnet && isPortOpen {
			tmplData = map[string]string{
				"Domain": hostName,
				"IP":     *instance.PublicIpAddress,
			}
		} else if jumphost != "" {
			tmplData = map[string]string{
				"Domain":    hostName,
				"IP":        *instance.PrivateIpAddress,
				"ProxyJump": jumphostHostname,
			}
		}

		if _, ok := tmplData["IP"]; ok {
			templateData.Instances = append(templateData.Instances, tmplData)
		}
	}

	t := template.Must(template.New("tmpl").Parse(tmplAWS))
	t.DefinedTemplates()
	err = t.Execute(os.Stdout, templateData)
	checkErr(err)
}
