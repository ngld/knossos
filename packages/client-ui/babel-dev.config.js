module.exports = {
  parserOpts: {
    strictMode: true,
  },
  presets: [
    ['@babel/preset-react', { runtime: 'automatic', development: true }],
    ['@babel/preset-typescript', { allowDeclareFields: true }],
    [
      '@babel/preset-env',
      {
        targets: 'chrome > 85',
        bugfixes: true,
        modules: false,
      },
    ],
  ],
  plugins: [
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    ['@babel/plugin-proposal-class-properties', { loose: false }],
    'react-refresh/babel',
  ],
};
