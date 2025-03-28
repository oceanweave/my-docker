
- 可能有描述不正确的地方，大体理解是对的
## 为什么要使用 cgo

- 对于 Mount Namespace 来说， 一个具有 多线程的进程是无法使用 setns 调用进入到对应的命名空间的。 但是， Go 每启动一个程序就会 进入多线程状态， 因此无法简简单单地在 Go 里面直接调用系统调用， 使当前的进程进入对应 的 Mount Namespace。 这里需要借助 C 来实现这个功能 。
- 是否可以将 go 进程变为单线程
  - 答 —— 不可以
    - 简单说，go runtime 是 GMP 模型，G 指的是 goroutine（用户级线程），M 指的是 内核级线程，P 是将 G 调度到 M 上来完成工作
    - 所以也就是说 Go 运行时默认多线程 —— 会启动 多个M（内核级线程）
    - 因此即使只有一个 goroutine，实现也是有多个 M —— 也就是多线程 —— 因此无法使用 setns
  - 因为 Go 运行时默认启动了多线程（多 goroutine 调度）
    - Go 语言的运行时会默认创建多个线程（M 结构），并在这些线程上调度 goroutines（G 结构）。
    - 即使你的 Go 代码看似只有一个 goroutine，Go 运行时仍可能会创建多个线程（特别是在 GOMAXPROCS > 1 时）。
    - 由于 Linux 规定 多线程进程不能用 setns 进入 Mount Namespace，所以 Go 进程很难直接调用 setns 进入目标命名空间。
  - Go 不能简单地 "停用" 其他线程
    - Go 运行时会动态管理线程，无法完全保证 setns 运行时只有一个线程。
    - 即使你在 main 线程中调用 runtime.LockOSThread() 绑定当前 goroutine 到一个线程，这个线程仍然属于一个多线程进程，所以 setns 仍然会失败。
- C 代码可以创建单线程进程
  - C 代码可以作为一个“包装器”来启动 Go 代码，确保 Go 进程在进入 Mount Namespace 之后再执行
    - C 代码（单线程进程）先进入 Mount Namespace（通过 setns）。
    - C 代码再 exec 启动 Go 代码（这样 Go 进程会在新的 Mount Namespace 内运行）
- 过程
  1. C 代码 setns 成功后，exec 启动 Go 进程，初始状态是单线程的。
  2. Go 进程在运行后可能会创建新的线程，但这不会影响它已经在新的 Mount Namespace 里。

## 如何使 C 代码包装 go 代码

1. 首先就是要保证 C 代码在 go 代码前执行（类似构造函数，程序运行时首先就会启动 C 代码）
2. 考虑个问题 —— 只有 exec 命令会需要使用 C 代码，而其他命令不需要 C 代码，C 代码的执行会影响该项目的其他命令
3. 解决方案 —— 在这段 C 代码前面一开始的位置就添加了环境变量检测，没有对应的环境变量时，就直接退出
   - 对于不使用 exec 功能的 Go 代码 ，只要不设置对应的环境变量 ， 那么当 C 程序检测到没有这个环境变量时，就会直接退出， 继续执行原来的代码， 并不会影响原来的逻辑。
   - 对于我们使用 exec 来说，当容器名和对应的命令传递进来以后，程序已经执行了，而且 那段 C 代码也应该运行完毕。那么，怎么指定环境变量让它再执行一遍呢？这里就用到了这 个／proc/self/exe 。这里又创建了一个 command ， 只不过这次只是简单地 fork 出来一个进程，不 需要这个进程拥有什么命名空间的隔离，然后把这个进程的标准输入输出都绑定到宿主机上 。 这样去 run 这里的进程时，实际上就是又运行了 一遍自己的程序，但是这时有一点不同的就是， 再一次运行的时候已经指定了环境变量，所以 C 代码执行的时候就能拿到对应的环境变量 ， 便可以进入到指定的 Namespace 中进行操作了
