local common = import 'common.libsonnet';
local grafana = import 'grafonnet-7.0/grafana.libsonnet';

local allLabels = 'job=~"$job", region=~"$region", dimension_DBInstanceIdentifier=~"$instance"';

grafana.dashboard.new(
  title='AWS RDS',
  description='Visualize Amazon RDS metrics',
  tags=['Amazon', 'AWS', 'CloudWatch', 'RDS'],
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
    query='label_values(aws_rds_database_connections_sum, job)',
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
    query='label_values(aws_rds_database_connections_sum, region)',
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
    query='label_values(aws_rds_database_connections_sum{job=~"$job", region=~"$region"}, dimension_DBInstanceIdentifier)',
    refresh=common.refreshOnTimeRangeChange,
    allValue='.+',
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
        Showing metrics only for AWS resources that have tags assigned to them. For more information, see [Amazon CloudWatch Metrics for Amazon RDS](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/monitoring-cloudwatch.html).
      |||,
    )
    .setGridPos(w=24, h=3),

    grafana.panel.graph.new(
      title='CPU utilization average',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_cpuutilization_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='CPU utilization maximum',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(
      format='percent',
      max=100,
      min=0,
    )
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_cpuutilization_maximum{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Database connections average',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_database_connections_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Database connections count',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_database_connections_sum{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Free storage space',
      datasource='$datasource',
    )
    .setGridPos(w=24, h=8)
    .addYaxis(format='bytes', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_free_storage_space_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Freeable memory',
      datasource='$datasource',
    )
    .setGridPos(w=24, h=8)
    .addYaxis(format='bytes', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_freeable_memory_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk read throughput (bytes)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='bps', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_read_throughput_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk write throughput (bytes)',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='bps', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_write_throughput_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk read IOPS',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='ops', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_read_iops_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk write IOPS',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='ops', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_write_iops_average{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk read latency',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8)
    .addYaxis(format='ms', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_read_latency_maximum{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),

    grafana.panel.graph.new(
      title='Disk write latency',
      datasource='$datasource',
    )
    .setGridPos(w=12, h=8, x=12)
    .addYaxis(format='ms', min=0)
    .addYaxis()
    .addTarget(
      grafana.target.prometheus.new(
        expr='aws_rds_write_latency_maximum{%s}' % [allLabels],
        legendFormat='{{dimension_DBInstanceIdentifier}}',
        datasource='$datasource',
      ),
    ),
  ]
)
