'use strict';

var Vue = require('vue');
var axios = require('axios');

module.exports = Vue.component('signin', function (resolve, reject) {
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
          // TODO implement get token process here
          window.localStorage.setItem('token', _this.email);
          _this.$router.push('/');
        }
      },
      mounted: function () {
        var _this = this;
        var token = window.localStorage.getItem('token');
        if (token) {
          // TODO if we have a good token, no need to be here
          _this.$router.push('/');
        }
        _this.state = 'signingIn';
      }
    });
  });
});
