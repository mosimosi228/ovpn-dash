<?php
    include_once './vendor/autoload.php';

    use OpenVpnPanel\Control\Commands;

    $clients = Commands::GetAllClients();

    $users = [];
    if (file_exists('./users.json')) {
        $users = json_decode(file_get_contents('./users.json'), true);
    }

    if (isset($_GET['delete']) && intval($_GET['delete']) === 1) {
        $user = $_GET['user'];
        if (isset($users[$user]) && count($users) > 1) {
            unset($users[$user]);
            file_put_contents('./users.json', json_encode($users, JSON_PRETTY_PRINT));
        }
    }

?>

<div class="container">
    <div align='center'><h3>Users</h3><br /></div>
    <table class="table table-bordered">
        <thead class="thead-dark" style="background: black;color: white;">
        <tr>
            <th scope="col">User</th>
            <th scope="col">Action</th>
        </tr>
        </thead>
        <tbody>
            <?php foreach ($users as $key => $user) { ?>
            <tr>
                <td><?php echo $key; ?></td>
                <td>
                    <a href="http://ovpn-new.iwad.ru/index.php?section=list_users&delete=1&user=<?php echo $key; ?>">delete</a>
                </td>
            </tr>
            <?php } ?>
        </tbody>
    </table>
</div><!-- /container -->
