var Vue = require('vue').default;
var VueRouter = require('vue-router').default;
Vue.use(VueRouter);

var components = {
  navbar: require('./components/navbar.vue').default,
  dashboard: require('./components/dashboard.vue').default,
  notFound: require('./components/not-found.vue').default,
  jobs: require('./components/jobs.vue').default,
  repos: require('./components/repos.vue').default,
  footer: require('./components/footer.vue').default
};

var router = new VueRouter({
  routes: [
    {
      path: '/',
      components: {
        header: components.navbar,
        main: components.dashboard,
        footer: components.footer
      }
    },
    {
      path: '/jobs',
      components: {
        header: components.navbar,
        main: components.jobs,
        footer: components.footer
      }
    },
    {
      path: '/repos',
      components: {
        header: components.navbar,
        main: components.repos,
        footer: components.footer
      }
    },
    {
      path: '*',
      components: {
        main: components.notFound
      }
    }
  ]
});

module.exports = router;
