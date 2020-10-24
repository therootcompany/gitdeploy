<template>
  <div class="jobs">
    <div class="container is-fluid">
      <div class="title">
        <h2 class="has-text-weight-bold">Jobs</h2>
      </div>
      <div v-if="jobs.length" class="jobs">
        <div v-for="j in jobs" class="job content">
          <pre>{{ j }}</pre>
        </div>
      </div>
      <template v-else>
        <div class="content">
          <p>There are no active jobs right now.</p>
        </div>
      </template>
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
      jobs: []
    };
  },
  methods: {
    getJobs: function () {
      var _this = this;
      return axios.get('/api/admin/jobs').then(function (resp) {
        _this.jobs = resp.data.jobs;
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
