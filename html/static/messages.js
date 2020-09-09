(function () {
  'use strict';

  var app = new Vue({
    data: {
      messages: []
    }
  });

  window.cdadmin = window.cdadmin || {};
  window.cdadmin.pushMessage = function (str) {
    window.alert(str);
  };
})();
