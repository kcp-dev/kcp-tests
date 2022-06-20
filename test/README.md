In order to make the `Golang` case running stable, please follow the below checkpoints:

- Please do not print any sensitive information, such as keys, credentials and customer information.
- Since the `Golang` is compiled program language, please compile your latest code before submitting it.
- To make sure your PR can be merged automatically, please develop your test case based on the latest code version.
- Please **clean up** the created resources no matter the case exits normal or not, the `Defer` is recommend.
- These `Golang` cases running parallelly by default, please don't use common names for your resources.
- The namespace created by the `oc.SetupProject()` will be removed automatically after the case running done.
- Please avoid using `oc.SetupNamespace()` to setup the namespace because it potentially impacts other case execution in parallel.
- Call the `exutil.FixturePath()` function in `g.It()`, not in `g.Describe()`.
- Output the logs as **less** as you can.
- For `g.Describe()`, please ensure set the correct `sub-team` name .
- For `g.It()`, please make sure your cases can be executed in parallel.
- When using the `wait.Poll`, please use the `exutil.AssertWaitPollNoErr()`, not the `o.Expect(err).NotTo(o.HaveOccurred())`
