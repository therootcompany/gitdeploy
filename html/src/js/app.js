'use strict';

require('../css/style.scss');

var Vue = require('vue');
var VueRouter = require('vue-router').default;
var axios = require('axios');

Vue.use(VueRouter);

var components = {
  signin: require('./components/signin.js'),
  navbar: require('./components/navbar.js'),
  dashboard: require('./components/dashboard.js'),
  notFound: require('./components/not-found.js')
};

var routes = [
  {
    path: '/',
    components: {
      header: components.navbar,
      main: components.dashboard
    }
  },
  {
    path: '/signin',
    components: {
      main: components.signin
    }
  },
  {
    path: '*',
    components: {
      main: components.notFound
    }
  }
];

var router = new VueRouter({ routes });

var app = new Vue({
  data: {
    user: {},
    ready: false
  },
  methods: {},
  watch: {
    $route: function (to, from) {
      var _this = this;
      this.ready = false;

      if (to.path.startsWith('/signin')) {
        _this.ready = true;
      } else {
        var token = window.localStorage.getItem('token');
        if (token) {
          _this.ready = true;
        } else {
          _this.$router.push('/signin');
        }
      }

      // console.log('route change', to, from);
    }
  },
  computed: {
    signedIn: function () {
      return Object.keys(this.user).length > 0;
    }
  },
  router,
  created: function () {}
});

app.$mount('.app');
