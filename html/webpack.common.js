'use strict';

var fs = require('fs');
var path = require('path');
var MiniCssExtractPlugin = require('mini-css-extract-plugin');

module.exports = {
  context: path.resolve(__dirname, 'src'),
  entry: {
    vendor: ['vue'],
    app: {
      import: './js/app.js',
      dependOn: 'vendor'
    }
  },
  output: {
    filename: `[name].js`
  },
  module: {
    rules: [
      {
        test: /\.s[ac]ss$/i,
        use: [
          MiniCssExtractPlugin.loader,
          // 'postcss-loader',
          'css-loader',
          'sass-loader'
        ]
      },
      {
        test: /\.(js|jsx)$/,
        exclude: /(node_modules|bower_components)/,
        use: {
          loader: 'babel-loader',
          options: {
            presets: ['@babel/preset-env']
          }
        }
      }
    ]
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: `style.css`
    })
  ],
  resolve: {
    alias: {
      bulma: path.resolve(__dirname, 'node_modules/bulma'),
      axios: path.resolve(__dirname, 'node_modules/axios/dist/axios.js'),
      vue: path.resolve(__dirname, 'node_modules/vue/dist/vue.js')
    }
  }
};
