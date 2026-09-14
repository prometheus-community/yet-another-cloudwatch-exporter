package v2

import (
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestApiGatewayResources(t *testing.T) {
	job := model.DiscoveryJob{Namespace: "AWS/ApiGateway"}
	rest := []apigatewaytypes.RestApi{
		{Id: aws.String("abc123"), Name: aws.String("orders")},
		{Id: aws.String("nameless")},
	}
	v2 := []apigatewayv2types.Api{{ApiId: aws.String("h1")}}

	got := apiGatewayResources(job, "cn-north-1", rest, v2)

	arns := make([]string, 0, len(got))
	for _, r := range got {
		arns = append(arns, r.ARN)
	}
	sort.Strings(arns)
	// Id-keyed like the tagging API, so FilterFunc renames REST ARNs and dedupes tagged duplicates.
	want := []string{
		"arn:aws-cn:apigateway:cn-north-1::/apis/h1",
		"arn:aws-cn:apigateway:cn-north-1::/restapis/abc123",
	}
	if len(arns) != len(want) {
		t.Fatalf("got %v, want %v", arns, want)
	}
	for i := range want {
		if arns[i] != want[i] {
			t.Errorf("arn[%d] = %q, want %q", i, arns[i], want[i])
		}
	}

	// Untagged resources cannot satisfy a searchTags job and must not appear.
	tagged := model.DiscoveryJob{Namespace: "AWS/ApiGateway", SearchTags: []model.SearchTag{{Key: "team"}}}
	if got := apiGatewayResources(tagged, "us-east-1", rest, v2); len(got) != 0 {
		t.Errorf("searchTags job returned %d untagged resources, want 0", len(got))
	}
}
