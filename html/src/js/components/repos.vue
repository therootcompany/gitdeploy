<template>
  <div class="sites">
    <div class="container is-fluid">
      <div class="title">
        <h2 class="has-text-weight-bold">Repositories</h2>
      </div>
      <pre>{{ repos }}</pre>
      <div class="block site" v-for="r in repos">
        <div class="title">
          <h3 class="has-text-weight-bold">{{ r.id }}</h3>
          <p>
            <button
              class="button is-primary"
              @click="promote($event, r.ref_name)"
            >
              <span>Promote</span>&nbsp;<b>{{ r.ref_name }}</b>
            </button>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
var axios = require('axios');

module.exports = {
  name: 'repos',
  data: function () {
    return {
      repos: []
    };
  },
  methods: {
    getRepos: function () {
      var _this = this;

      return axios.get('/api/admin/repos').then(function (resp) {
        if (resp.data.success) {
          _this.repos = resp.data.repos;
        }
      });
    },
    promote: function (ev, ref_name) {
      if (ev) {
        ev.preventDefault();
      }

      if (window.confirm('Are you sure you want to promote master?')) {
        axios.post('/api/admin/promote', {
          clone_url: 'https://...',
          ref_name: ref_name
        });
      }
    }
  },
  created: function () {
    var _this = this;

    _this.getRepos();
  }
};
</script>
