local common = import 'common.libsonnet';
local grafana = import 'grafonnet-7.0/grafana.libsonnet';

local allLabels = 'job=~"$job", region=~"$region", dimension_FunctionName=~"$function_name", dimension_Resource=~"$resource", dimension_ExecutedVersion=~"$executed_version"';

grafana.dashboard.new(
  title='AWS Lambda',
  description='Visualize Amazon Lambda metrics',
  tags=['Amazon', 'AWS', 'CloudWatch', 'Lambda'],
  graphTooltip=common.tooltipSharedCrosshair,
)
.addTemplate(
  grafana.template.datasource.new(
    name='datasource',
    query='prometheus',
    label='Data Source',
  )
)
.addTemplate(
  grafana.template.query.new(
    name='job',
    label='job',
    datasource='$datasource',
    query='label_values(aws_lambda_invocations_sum, job)',
    refresh=common.refreshOnPageLoad,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
    allValue='.+',
  )
)
.addTemplate(
  grafana.template.query.new(
    name='region',
    label='Region',
    datasource='$datasource',
    query='label_values(aws_lambda_invocations_sum, region)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='function_name',
    label='Function name',
    datasource='$datasource',
    query='label_values(aws_lambda_invocations_sum{job=~"$job", region=~"$region"}, dimension_FunctionName)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
    allValue='.+',
  )
)
.addTemplate(
  grafana.template.query.new(
    name='resource',
    label='Resource',
    datasource='$datasource',
    query='label_values(aws_lambda_invocations_sum{job=~"$job", region=~"$region", dimension_FunctionName=~"$function_name"}, dimension_Resource)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='executed_version',
    label='Executed Version',
    datasource='$datasource',
    query='label_values(aws_lambda_invocations_sum{job=~"$job", region=~"$region", dimension_FunctionName=~"$function_name", dimension_Resource=~"$resource"}, dimension_ExecutedVersion)',
    refresh=common.refreshOnTimeRangeChange,
    allValue='.*',
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addPanels(
  [
    grafana.panel.text.new(
      title='Info',
      content=|||
        Showing metrics only for AWS resources that have tags assigned to them. For more information, see [Amazon CloudWatch Metrics for Amazon Lambda](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-metrics.html).
      |||,
    )
    .setGridPos(w=24, h=3),

    grafana.panel.graph.new(
      title='Invocations',
      description='The number of times your function code is executed.',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='short', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_invocations_sum{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Errors',
      description='The number of invocations that result in a function error.',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='short', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_errors_sum{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Throttles',
      description='The number of invocation requests that are throttled.',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='short', min=0, decimals=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_throttles_sum{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Duration',
      description='The time that your function code spends processing an event.',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='ms', min=0, decimals=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_duration_p90{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}} (p90)',
        datasource='$datasource',
      ),
    )
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_duration_minimum{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}} (min)',
        datasource='$datasource',
      ),
    )
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum by (dimension_FunctionName) (aws_lambda_duration_maximum{%s})' % [allLabels],
        legendFormat='{{dimension_FunctionName}} (max)',
        datasource='$datasource',
      ),
    ),
  ]
)
