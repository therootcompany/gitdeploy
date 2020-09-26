'use strict';

var Vue = require('vue');
var axios = require('axios');

module.exports = Vue.component('not-found', function (resolve, reject) {
  axios.get('/templates/not-found.html').then(function (resp) {
    resolve({
      template: resp.data
    });
  });
});
