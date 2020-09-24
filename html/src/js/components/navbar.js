'use strict';

var Vue = require('vue');
var axios = require('axios');

module.exports = Vue.component('navbar', function (resolve, reject) {
  return axios.get('/templates/navbar.html').then(function (resp) {
    resolve({
      template: resp.data,
      data: function () {
        return {};
      }
    });
  });
});
