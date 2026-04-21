package monitor

import (
	"time"

	"github.com/beego/beego/v2/client/orm"

	"github.com/OZON08/openvpn-ui/models"
)

// AggregateSamplesToHourly rolls samples older than sampleRetentionDays into
// per-(common_name, hour) deltas and appends them to TrafficHourly. Rows
// already aggregated are identified by a (common_name, hour_ts) UPSERT.
//
// Deltas are computed per session: for each (session_id, hour) bucket we
// take MAX(bytes)-MIN(bytes); since byte counters are monotonic within a
// session, this yields the transfer volume that happened inside that hour.
func AggregateSamplesToHourly(sampleRetentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -sampleRetentionDays)
	o := orm.NewOrm()

	const aggSQL = `
		INSERT INTO traffic_hourly (common_name, hour_ts, bytes_in_delta, bytes_out_delta, session_count)
		SELECT
			common_name,
			strftime('%Y-%m-%d %H:00:00', sampled_at) AS hour_ts,
			SUM(in_delta)  AS bytes_in_delta,
			SUM(out_delta) AS bytes_out_delta,
			COUNT(DISTINCT session_id) AS session_count
		FROM (
			SELECT
				common_name,
				session_id,
				sampled_at,
				MAX(bytes_in)  - MIN(bytes_in)  AS in_delta,
				MAX(bytes_out) - MIN(bytes_out) AS out_delta
			FROM traffic_sample
			WHERE sampled_at < ?
			GROUP BY session_id, strftime('%Y-%m-%d %H', sampled_at)
		)
		GROUP BY common_name, hour_ts
	`
	if _, err := o.Raw(aggSQL, cutoff).Exec(); err != nil {
		return err
	}
	return nil
}

// AggregateHourlyToDaily rolls TrafficHourly rows older than hourlyRetentionDays
// into TrafficDaily and returns.
func AggregateHourlyToDaily(hourlyRetentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -hourlyRetentionDays)
	o := orm.NewOrm()

	const aggSQL = `
		INSERT INTO traffic_daily (common_name, day_ts, bytes_in_delta, bytes_out_delta, session_count)
		SELECT
			common_name,
			date(hour_ts)    AS day_ts,
			SUM(bytes_in_delta),
			SUM(bytes_out_delta),
			SUM(session_count)
		FROM traffic_hourly
		WHERE hour_ts < ?
		GROUP BY common_name, date(hour_ts)
	`
	if _, err := o.Raw(aggSQL, cutoff).Exec(); err != nil {
		return err
	}
	return nil
}

// PruneOldSamples deletes TrafficSample rows older than the retention window.
// Call AFTER AggregateSamplesToHourly so the data is preserved at coarser grain.
func PruneOldSamples(sampleRetentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -sampleRetentionDays)
	_, err := orm.NewOrm().QueryTable(new(models.TrafficSample)).
		Filter("SampledAt__lt", cutoff).
		Delete()
	return err
}

// PruneOldHourly deletes TrafficHourly rows older than the retention window.
// Call AFTER AggregateHourlyToDaily.
func PruneOldHourly(hourlyRetentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -hourlyRetentionDays)
	_, err := orm.NewOrm().QueryTable(new(models.TrafficHourly)).
		Filter("HourTs__lt", cutoff).
		Delete()
	return err
}
