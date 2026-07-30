# 二次开发分支流程

本仓库使用以下分支职责：

- `main`：保持与上游项目的代码基线一致，避免直接进行二次开发。
- `publish`：二次开发的稳定集成分支，所有已验证的自定义功能最终合并到这里。
- `feature/<功能名>`：从 `publish` 创建的具体功能开发分支。

## 开发功能

```bash
git switch publish
git switch -c feature/<功能名>
# 开发、提交
git switch publish
git merge feature/<功能名>
```

## 同步上游项目

先确认工作区没有未提交的修改，再将上游最新代码合并到 `publish`：

```bash
git switch publish
git fetch upstream
git merge upstream/main
```

如发生冲突，人工整合当前二开逻辑与上游改动，随后执行：

```bash
git add <已解决的文件>
git commit
```

合并完成后应执行相关测试，再将 `publish` 推送到个人远程仓库：

```bash
git push -u origin publish
```

建议定期、小批量同步上游，避免长期积累冲突。
