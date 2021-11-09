# 贡献指南

## 仓库设置

有兴趣的开发者请通过fork & pull request的方式来贡献代码:

1. FORK 点击项目右上角的fork按钮，把项目代码 fork到自己的仓库中如 farmerluo/cloud-provider-tencent
2. CLONE 把fork后的项目clone到自己本地，如`git clone https://github.com/farmerluo/cloud-provider-tencent`
3. Set Remote upstream, 方便把weimob-tech/cloud-provider-tencent的代码更新到自己的仓库中

```shell script
git remote add upstream https://github.com/weimob-tech/cloud-provider-tencent.git
git remote set-url --push upstream no-pushing
```

更新主分支代码到自己的仓库可以：

```shell script
git pull upstream main # git pull <remote name> <branch name>
git push
```

建议main分支只做代码同步，所有功能切一个新分支开发，如修复一个bug:

```shell script
git checkout -b bugfix/cloud-provider
# 开发完成后
git push --set-upstream origin bugfix/cloud-provider
```

代码先提交到自己的仓库中，然后在自己的仓库里点击pull request申请合并代码。
然后在MR中就可以看到一个黄色的 "signed the CLA" 按钮，点一下签署CLA直至按钮变绿.

## 代码开发

代码的commit信息请尽量描述清楚, 一个PR功能也尽可能单一一些方便review

### 需求开发

如果你有一些新的需求，建议先开issue讨论，再进行编码开发。

### bug修复以及优化

任何优化的点都可以PR，如文档不全，发现bug，排版问题，多余空格，错别字，健壮性处理，冗长代码重复代码，命名不规范, 丑陋代码等等