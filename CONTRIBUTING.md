## How to contribute to MinBFT

Any kind of feedback is highly appreciated. Questions, suggestions,
bug reports, change requests, etc. are welcome.

### Questions, suggestions, bug reports

If you would like to report a problem or open up a discussion, please
[file a GitHub issue][new-issue].

[new-issue]: https://github.com/hyperledger-labs/minbft/issues/new

### Submitting changes

In case you would like to submit changes to code or documentation, you
should [submit a pull request][new-pr].

We agreed on the following convention to conduct code review process
and manage pull requests. [Draft pull requests][about-draft-pr] are
not subject to strictly follow the convention, though.

[about-draft-pr]: https://help.github.com/en/articles/about-pull-requests#draft-pull-requests

#### Merge rules

A pull request must be reviewed by at least one member of [this GitHub
team][minbft-committers], excluding the request author. The review
process is open; anyone is welcome to provide constructive comments.
Additionally, pull requests are required to pass some automated
checks, namely check for [Developer Certificate of Origin (DCO)][dco]
and continuous integration. The request may be merged once the checks
pass and all the review comments are resolved.

[minbft-committers]: https://github.com/orgs/hyperledger-labs/teams/minbft-committers
[dco]: https://developercertificate.org/

#### Creating pull requests

A new pull request should originate from a branch (e.g. some-feature)
in a forked repository.

Pull requests need to maintain good quality of the code base. Thus,
they need to pass automated tests, as well as code analysis (linting).
The following command helps to run tests and linting locally.

    make test lint

For the sake of efficient review and to maintain usable history of
changes, commit series constituting a pull request should comprise a
coherent story telling how the proposed bigger change is achieved in
smaller, consistent steps.

[new-pr]: https://github.com/hyperledger-labs/minbft/compare

When a pull request is created, reviewers are automatically set and one
of the reviewers assigns the pull request to themselves. The assignee
is responsible for merging the request. Note that, if the author is a
maintainer of this project, the author takes the responsibility.

The reviewers give their feedback or explicitly ask for more time in a
couple of working days. Generally, the reviewers make a response within
a week.

#### Addressing review comments

After review comments are collected, they should be addressed,
preferably by _directly changing the corresponding commits_ (using
`git rebase`). The updated revision of proposed changes should
normally be force-pushed to the original branch of the pull request.
The pull request update should be followed by a comment shortly
summarising the changes.

### Code of conduct

We expect respectful, constructive communication and adopt the
[Hyperledger Project Code of Conduct][code-of-conduct].

[code-of-conduct]: https://wiki.hyperledger.org/community/hyperledger-project-code-of-conduct
