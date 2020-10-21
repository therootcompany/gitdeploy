<template>
	<div class="jobs">
		<div class="container is-fluid">
			<div class="title">
				<h2 class="has-text-weight-bold">Jobs</h2>
			</div>
			<pre>{{ jobs }}</pre>
		</div>
	</div>
</template>

<script>
var axios = require('axios');
var { setIntervalAsync } = require('set-interval-async').dynamic;
var { clearIntervalAsync } = require('set-interval-async');

module.exports = {
	name: 'jobs',
	data: function () {
		return {
			poller: null,
			jobs: {}
		};
	},
	methods: {
		getJobs: function () {
			var _this = this;
			return axios.get('/api/admin/jobs').then(function (resp) {
				_this.jobs = resp.data;
			});
		}
	},
	mounted: function () {
		var _this = this;
		_this.getJobs();
		_this.poller = setIntervalAsync(function () {
			_this.getJobs();
		}, 1000);
	},
	destroyed: function () {
		var _this = this;
		clearIntervalAsync(_this.poller);
	}
};
</script>
