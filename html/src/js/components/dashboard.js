'use strict';

var Vue = require('vue');
var axios = require('axios');

module.exports = Vue.component('dashboard', function (resolve, reject) {
  return axios.get('/templates/dashboard.html').then(function (resp) {
    resolve({
      template: resp.data,
      data: function () {
        return {};
      },
      props: ['user'],
      methods: {},
      mounted: function () {
        var _this = this;
        _this.ready = true;
      }
    });
  });
});
