'use strict';

var fs = require('fs');
var path = require('path');
var MiniCssExtractPlugin = require('mini-css-extract-plugin');
var VueLoaderPlugin = require('vue-loader/lib/plugin');

module.exports = {
	context: path.resolve(__dirname, 'src'),
	entry: {
		vendor: ['vue', 'vue-router', 'axios'],
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
				test: /\.(js)$/,
				exclude: /(node_modules|bower_components)/,
				use: {
					loader: 'babel-loader',
					options: {
						presets: ['@babel/preset-env']
					}
				}
			},
			{
				test: /\.vue$/,
				loader: 'vue-loader'
			}
		]
	},
	plugins: [
		new MiniCssExtractPlugin({
			filename: `style.css`
		}),
		new VueLoaderPlugin()
	],
	resolve: {
		alias: {
			bulma: path.resolve(__dirname, 'node_modules/bulma'),
			axios: path.resolve(__dirname, 'node_modules/axios/dist/axios.js'),
			vue$: 'vue/dist/vue.esm.js'
		}
	}
};
