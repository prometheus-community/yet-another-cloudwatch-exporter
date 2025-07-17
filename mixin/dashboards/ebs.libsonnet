local common = import 'common.libsonnet';
local grafana = import 'grafonnet-7.0/grafana.libsonnet';

local allLabels = 'job=~"$job", region=~"$region", dimension_VolumeId=~"$volume"';

grafana.dashboard.new(
  title='AWS EBS',
  description='Visualize Amazon EBS metrics',
  tags=['Amazon', 'AWS', 'CloudWatch', 'EBS'],
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
    query='label_values(aws_ebs_volume_idle_time_average, job)',
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
    query='label_values(aws_ebs_volume_idle_time_average, region)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='volume',
    label='Volume',
    datasource='$datasource',
    query='label_values(aws_ebs_volume_idle_time_average{job=~"$job", region=~"$region"}, dimension_VolumeId)',
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
        Showing metrics only for AWS resources that have tags assigned to them. For more information, see [Amazon CloudWatch Metrics for Amazon EBS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using_cloudwatch_ebs.html).
      |||,
    )
    .setGridPos(w=24, h=3),

    grafana.panel.graph.new(
      title='Volume read bandwidth (bytes)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='bps', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_read_bytes_sum{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume write bandwidth (bytes)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='bps', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_write_bytes_sum{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume read throughput (operations)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=0, y=8)
    .addYaxis(format='ops', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_read_ops_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume write throughput (operations)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12, y=8)
    .addYaxis(format='ops', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_write_ops_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume idle time',
      datasource='$datasource',
    )
    .setGridPos(w=8, h=8, x=0, y=16)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_idle_time_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume total read time',
      datasource='$datasource',
    )
    .setGridPos(w=8, h=8, x=8, y=16)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_total_read_time_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume total write time',
      datasource='$datasource',
    )
    .setGridPos(w=8, h=8, x=16, y=16)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_total_write_time_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume queue length (bytes)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=0, y=24)
    .addYaxis(format='short', min=0, max=1)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_queue_length_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume throughput percentage',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12, y=24)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_throughput_percentage_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),


    grafana.panel.graph.new(
      title='Burst balance',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=0, y=32)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_burst_balance_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Volume consumed r/w operations',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12, y=32)
    .addYaxis(format='short')
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ebs_volume_consumed_read_write_ops_average{%s}' % [allLabels],
        legendFormat='{{dimension_VolumeId}}',
        datasource='$datasource',
      ),
    ),

  ]
)
