<template>
  <div class="x-footer">
    <div class="container is-fluid">
      <div class="columns">
        <div class="column x-l">
          <p>
            <a
              href="https://github.com/therootcompany/gitdeploy"
              target="_blank"
            >
              gitdeploy
            </a>
            v{{ version }}
          </p>
        </div>
        <div class="column x-r">
          <p>
            <button v-if="signedIn" class="button is-small">Sign Out</button>
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
var axios = require("axios");

module.exports = {
  name: "footer",
  data: function () {
    return {
      version: "",
    };
  },
  methods: {
    getVersion: function () {
      var _this = this;

      axios.get("/api/version").then(function (resp) {
        _this.version = resp.data.version;
      });
    },
  },
  created: function () {
    var _this = this;

    _this.getVersion();
  },
};
</script>
