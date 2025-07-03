local common = import 'common.libsonnet';
local grafana = import 'grafonnet-7.0/grafana.libsonnet';

local allLabels = 'job=~"$job", region=~"$region", dimension_BucketName=~"$bucket"';

grafana.dashboard.new(
  title='AWS S3',
  description='Visualize Amazon S3 metrics',
  tags=['Amazon', 'AWS', 'CloudWatch', 'S3'],
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
    query='label_values(aws_s3_number_of_objects_average, job)',
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
    query='label_values(aws_s3_number_of_objects_average, region)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='bucket',
    label='Bucket',
    datasource='$datasource',
    query='label_values(aws_s3_number_of_objects_average, dimension_BucketName)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='filter_id',
    label='FilterId',
    datasource='$datasource',
    query='label_values(aws_s3_all_requests_sum{dimension_BucketName=~"$bucket"}, dimension_FilterId)',
    refresh=common.refreshOnTimeRangeChange,
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
        Showing metrics only for AWS resources that have tags assigned to them. For more information, see [Amazon CloudWatch Metrics for Amazon S3](https://docs.aws.amazon.com/AmazonS3/latest/userguide/metrics-dimensions.html).
      |||,
    )
    .setGridPos(w=24, h=3),

    grafana.panel.stat.new(
      title='Total number of objects',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=4, x=0, y=3)
    .setFieldConfig(min=0)
    .setOptions(calcs=['lastNotNull'], colorMode='none')
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum(last_over_time(aws_s3_number_of_objects_average{job=~"$job"}[1d]) > 0)',
        datasource='$datasource',
      ),
    ),

    grafana.panel.stat.new(
      title='Total buckets size',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=4, x=12, y=3)
    .setFieldConfig(unit='bytes', min=0)
    .setOptions(calcs=['lastNotNull'], colorMode='none')
    .addTarget(
      grafana.target.prometheus.new(
        expr='sum(last_over_time(aws_s3_bucket_size_bytes_average{job=~"$job"}[1d]) > 0)',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Number of objects',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=0, y=7)
    .addYaxis(format='short', min=0, decimals=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='last_over_time(aws_s3_number_of_objects_average{%s}[1d])' % [allLabels],
        legendFormat='{{dimension_BucketName}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Bucket size',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12, y=7)
    .addYaxis(format='bytes', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='last_over_time(aws_s3_bucket_size_bytes_average{%s}[1d])' % [allLabels],
        legendFormat='{{dimension_BucketName}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.row.new(
      title='Request metrics',
      datasource='$datasource',
    )
    .setGridPos(w=24, h=1, x=0, y=15)
    .addPanel(
      grafana.panel.text.new(
        title='Info',
        content=|||
          Enable [Requests metrics](https://docs.aws.amazon.com/AmazonS3/latest/userguide/cloudwatch-monitoring.html) from the AWS console and create a Filter to make sure your requests metrics are reported.
        |||,
      )
      .setGridPos(w=24, h=2, x=0, y=16),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Request latency (p95)',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=0, y=18)
      .addYaxis(format='ms', min=0, decimals=1)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='rate(aws_s3_total_request_latency_p95{%s, dimension_FilterId=~"$filter_id"}[2h]) * 1e3' % [allLabels],
          legendFormat='{{dimension_BucketName}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Errors count',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=12, y=18)
      .addYaxis(format='short', min=0, decimals=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_s3_4xx_errors_sum{%s, dimension_FilterId=~"$filter_id"}' % [allLabels],
          legendFormat='{{dimension_BucketName}}',
          datasource='$datasource',
        ),
      ),
    ),

  ]
)
