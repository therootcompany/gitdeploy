"use strict";

require("../css/style.scss");

var Vue = require("vue").default;
var app = require("./app.vue").default;

new Vue({
  el: "#app",
  components: {
    app: app,
  },
});
