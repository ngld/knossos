module.exports = {
  parserOpts: {
    strictMode: true,
  },
  presets: [
    [
      '@babel/preset-env',
      {
        targets: '> 0.5%, not IE 11, firefox esr',
      },
    ],
    ['@babel/preset-react', { runtime: 'automatic' }],
    ['@babel/preset-typescript', { allowDeclareFields: true }],
  ],
  plugins: [
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    ['@babel/plugin-proposal-class-properties', { loose: true }],
    '@babel/plugin-transform-react-inline-elements',
    '@babel/plugin-transform-react-constant-elements',
  ],
};
