local common = import 'common.libsonnet';
local grafana = import 'grafonnet-7.0/grafana.libsonnet';

local allLabels = 'job=~"$job", region=~"$region", dimension_InstanceId=~"$instance"';

grafana.dashboard.new(
  title='AWS EC2',
  description='Visualize Amazon EC2 metrics',
  tags=['Amazon', 'AWS', 'CloudWatch', 'EC2'],
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
    query='label_values(aws_ec2_cpuutilization_maximum, job)',
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
    query='label_values(aws_ec2_cpuutilization_maximum, region)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
  )
)
.addTemplate(
  grafana.template.query.new(
    name='instance',
    label='instance',
    datasource='$datasource',
    query='label_values(aws_ec2_cpuutilization_maximum{job=~"$job", region=~"$region"}, dimension_InstanceId)',
    refresh=common.refreshOnTimeRangeChange,
    includeAll=true,
    multi=true,
    sort=common.sortAlphabeticalAsc,
    allValue='.+',
  )
)
.addPanels(
  [
    grafana.panel.text.new(
      title='Info',
      content=|||
        Showing metrics only for AWS resources that have tags assigned to them. For more information, see [Amazon CloudWatch Metrics for Amazon EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/viewing_metrics_with_cloudwatch.html).
      |||,
    )
    .setGridPos(w=24, h=3),

    grafana.panel.graph.new(
      title='CPU utilization',
      datasource='$datasource',
    )
    .setGridPos(w=24, h=8, x=0, y=3)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ec2_cpuutilization_maximum{%s}' % [allLabels],
        legendFormat='{{dimension_InstanceId}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Average network traffic',
      datasource='$datasource',
    )
    .setGridPos(w=24, h=8, x=0, y=11)
    .addYaxis(
      format='bps',
      label='bytes in (+) / out (-)'
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ec2_network_in_average{%s}' % [allLabels],
        legendFormat='{{dimension_InstanceId}} inbound',
        datasource='$datasource',
      ),
    )
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_ec2_network_out_average{%s}' % [allLabels],
        legendFormat='{{dimension_InstanceId}} outbound',
        datasource='$datasource',
      ),
    )
    .addSeriesOverride(alias='/.*outbound/', transform='negative-Y'),

    grafana.panel.row.new(
      title='Network details',
    )
    .setGridPos(w=12, h=16, x=0, y=19)
    .addPanel(
      grafana.panel.graph.new(
        title='Inbound network traffic',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=0, y=19)
      .addYaxis(
        format='bps',
        min=0,
      )
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_network_in_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Outbound network traffic',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=12, y=19)
      .addYaxis(format='bps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_network_out_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Inbound network packets',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=0, y=27)
      .addYaxis(format='pps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_network_packets_in_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Outbound network packets',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=12, y=27)
      .addYaxis(format='pps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_network_packets_out_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    ),

    grafana.panel.row.new(
      title='Disk details',
    )
    .setGridPos(w=24, h=18, x=0, y=35)
    .addPanel(
      grafana.panel.text.new(
        content='The following metrics are reported for EC2 Instance Store Volumes. For Amazon EBS volumes, see the EBS dashboard.',
      )
      .setGridPos(w=24, h=2, x=0, y=35),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Disk reads (bytes)',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=0, y=37)
      .addYaxis(format='bps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_disk_read_bytes_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Disk writes (bytes)',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=12, y=37)
      .addYaxis(format='bps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_disk_write_bytes_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Disk read (operations)',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=0, y=45)
      .addYaxis(format='pps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_disk_read_ops_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Disk write (operations)',
        datasource='$datasource',
      )
      .setGridPos(w=12, h=8, x=12, y=45)
      .addYaxis(format='pps', min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_disk_write_ops_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    ),

    grafana.panel.row.new(
      title='Status checks',
    )
    .setGridPos(w=24, h=8, x=0, y=53)
    .addPanel(
      grafana.panel.graph.new(
        title='Status check failed (system)',
        datasource='$datasource',
      )
      .setGridPos(w=8, h=8, x=0, y=53)
      .addYaxis(min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_status_check_failed_system_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Status check failed (instance)',
        datasource='$datasource',
      )
      .setGridPos(w=8, h=8, x=8, y=53)
      .addYaxis(min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_status_check_failed_instance_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    )
    .addPanel(
      grafana.panel.graph.new(
        title='Status check failed (all)',
        datasource='$datasource',
      )
      .setGridPos(w=8, h=8, x=16, y=53)
      .addYaxis(min=0)
      .addYaxis()
      .addTarget(
        grafana.target.prometheus.new(
          expr='aws_ec2_status_check_failed_sum{%s}' % [allLabels],
          legendFormat='{{dimension_InstanceId}}',
          datasource='$datasource',
        ),
      ),
    ),
  ],
)
