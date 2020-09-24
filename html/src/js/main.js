'use strict';

require('../css/style.scss');

var Vue = require('vue');
var VueRouter = require('vue-router').default;
var axios = require('axios');

Vue.use(VueRouter);

var components = {};

components.signin = Vue.component('signin', function (resolve, reject) {
  return axios.get('/templates/signin.html').then(function (resp) {
    resolve({
      template: resp.data,
      data: function () {
        return {
          email: '',
          busy: false,
          state: ''
        };
      },
      methods: {
        signin: function (ev) {
          ev.preventDefault();
          ev.stopPropagation();
          var _this = this;
          _this.busy = true;
          //console.log(_this.email);
          window.localStorage.setItem('token', _this.email);
        }
      },
      beforeMount: function () {
        console.log('beforeMount signin');
      },
      mounted: function () {
        // var _this = this;
        // var token = window.localStorage.getItem('token');
        // if (token) {
        //   getMe().then(function (result) {
        //     if (result) {
        //       router.push('/');
        //     } else {
        //       _this.state = 'signingIn';
        //     }
        //   });
        // } else {
        //   _this.state = 'signingIn';
        // }
      }
    });
  });
});

components.navbar = Vue.component('navbar', function (resolve, reject) {
  return axios.get('/templates/navbar.html').then(function (resp) {
    resolve({
      template: resp.data,
      data: function () {
        return {
          ready: false
        };
      },
      mounted: function () {
        var _this = this;
        _this.$parent.getUser().then(function (user) {
          if (user.email) {
            _this.ready = true;
          } else {
            _this.$router.push('/signin');
          }
        });
      }
    });
  });
});

components.dashboard = Vue.component('dashboard', function (resolve, reject) {
  return axios.get('/templates/dashboard.html').then(function (resp) {
    resolve({
      template: resp.data,
      data: function () {
        return {};
      },
      props: ['user'],
      computed: {
        ready: function () {
          return !!this.user.email;
        }
      },
      methods: {}
    });
  });
});

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
  }
];

var router = new VueRouter({
  routes
});

var app = new Vue({
  data: {
    user: {}
  },
  methods: {
    getUser: function () {
      var _this = this;
      if (_this.user) {
        return Promise.resolve(_this.user);
      }
      return getMe().then(function (result) {
        _this.user = result.user;
        return _this.user;
      });
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

function getMe() {
  var token = window.localStorage.getItem('token');
  if (!token) {
    return Promise.resolve(false);
  }
  return axios
    .get('/api/auth/me.json')
    .then(function (resp) {
      return {
        token,
        user: resp.data
      };
    })
    .catch(function (err) {
      if (err.resp.status === 401) {
        return false;
      }
      throw err;
    });
}
