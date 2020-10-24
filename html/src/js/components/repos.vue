<template>
  <div class="sites">
    <div class="container is-fluid">
      <div class="title">
        <h2 class="has-text-weight-bold">Repositories</h2>
      </div>
      <div class="box site repo-item" v-for="r in repos">
        <div class="title">
          <h3 class="has-text-weight-bold is-size-4">
            <a :href="r.clone_url" target="_blank">{{ r.id }}</a>
          </h3>
        </div>
        <div v-if="false" class="block content">
          <h4 class="is-size-5">Deploy</h4>
          <p class="buttons">
            <button class="button" @click="deploy($event, r)">Deploy</button>
          </p>
        </div>
        <div class="block content">
          <h4 class="is-size-5">Promote</h4>
          <p class="buttons">
            <button
              class="button"
              @click="promote($event, r, p[0], p[1])"
              v-for="p in promotions(r)"
            >
              <span>
                Promote <b>{{ p[0] }}</b> to <b>{{ p[1] }}</b>
              </span>
            </button>
          </p>
        </div>
        <!-- <pre>{{ r }}</pre> -->
        <!-- <pre>{{ promotions(r) }}</pre> -->
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
    promotions: function (repo) {
      return repo._promotions
        .map(function (el, i) {
          if (i < repo._promotions.length - 1) {
            return [repo._promotions[i + 1], el];
          }
        })
        .filter(function (el) {
          return !!el;
        })
        .reverse();
    },
    // deploy: function (ev, cloneUrl, branch) {
    //   if (ev) {
    //     ev.preventDefault();
    //   }

    //   if (window.confirm('Are you sure you want to deploy ' + cloneUrl + '?')) {
    //   }
    // },
    promote: function (ev, repo, from, to) {
      var _this = this;
      if (ev) {
        ev.preventDefault();
      }

      if (
        window.confirm(
          'Are you sure you want to promote ' +
            repo.id +
            ' from ' +
            from +
            ' to ' +
            to +
            '?'
        )
      ) {
        axios
          .post('api/admin/promote', {
            clone_url: repo.clone_url,
            ref_name: from
          })
          .then(function () {
            _this.$router.push('/jobs');
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
