package php_test

// data_extractors_test.go — tests for orm_data.go, driver_sql.go, test_data.go
// Coverage: Doctrine, Eloquent, CycleORM, Propel, RedBeanPHP schema/rel/migration;
// MySQL/Postgres/SQLite driver schema; Behat/Codeception/Pest test extractors.

import "testing"

// ============================================================================
// Doctrine ORM
// ============================================================================

func TestDoctrineColumn(t *testing.T) {
	src := `<?php
namespace App\Entity;
use Doctrine\ORM\Mapping as ORM;

#[ORM\Entity(repositoryClass: UserRepository::class)]
class User
{
    #[ORM\Column(type: 'string', length: 255)]
    private string $username;

    #[ORM\Column(type: 'string', unique: true)]
    private string $email;
}
`
	ents := extract(t, "php_doctrine_orm_data", fi("User.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "username") {
		t.Error("expected username column schema")
	}
	if !containsEntity(ents, "SCOPE.Schema", "email") {
		t.Error("expected email column schema")
	}
}

func TestDoctrineRelationship(t *testing.T) {
	src := `<?php
#[ORM\Entity]
class Order
{
    #[ORM\ManyToOne(targetEntity: Customer::class, inversedBy: 'orders')]
    private Customer $customer;

    #[ORM\OneToMany(targetEntity: OrderItem::class, mappedBy: 'order')]
    private Collection $items;
}
`
	ents := extract(t, "php_doctrine_orm_data", fi("Order.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "relation:ManyToOne") {
		t.Error("expected ManyToOne relation component")
	}
	if !containsEntity(ents, "SCOPE.Component", "relation:OneToMany") {
		t.Error("expected OneToMany relation component")
	}
}

func TestDoctrineFK(t *testing.T) {
	src := `<?php
#[ORM\Entity]
class OrderItem
{
    #[ORM\ManyToOne]
    #[ORM\JoinColumn(nullable: false)]
    private Order $order;
}
`
	ents := extract(t, "php_doctrine_orm_data", fi("OrderItem.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "join_column") {
		t.Error("expected join_column FK entity")
	}
}

func TestDoctrineLazyLoading(t *testing.T) {
	src := `<?php
#[ORM\Entity]
class Product
{
    #[ORM\OneToMany(fetch: 'LAZY', targetEntity: Review::class)]
    private Collection $reviews;
}
`
	ents := extract(t, "php_doctrine_orm_data", fi("Product.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "lazy:LAZY") {
		t.Error("expected lazy:LAZY pattern entity")
	}
}

func TestDoctrineMigration(t *testing.T) {
	src := `<?php
use Doctrine\Migrations\AbstractMigration;

class Version20230101120000 extends AbstractMigration
{
    public function up(Schema $schema): void
    {
        $this->addSql('CREATE TABLE user (id INT NOT NULL)');
    }

    public function down(Schema $schema): void
    {
        $this->addSql('DROP TABLE user');
    }
}
`
	ents := extract(t, "php_doctrine_orm_data", fi("Version20230101120000.php", "php", src))
	if !containsEntity(ents, "SCOPE.Operation", "Version20230101120000") {
		t.Error("expected migration class entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "migration:up") {
		t.Error("expected migration:up entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "migration:down") {
		t.Error("expected migration:down entity")
	}
}

func TestDoctrineNoMatch(t *testing.T) {
	src := `<?php echo "hello doctrine";`
	ents := extract(t, "php_doctrine_orm_data", fi("plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// Eloquent ORM
// ============================================================================

func TestEloquentFillableSchema(t *testing.T) {
	src := `<?php
use Illuminate\Database\Eloquent\Model;

class Post extends Model
{
    protected $fillable = ['title', 'content', 'published_at'];
    protected $casts = [
        'published_at' => 'datetime',
        'is_active' => 'boolean',
    ];
}
`
	ents := extract(t, "php_eloquent_orm_data", fi("Post.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "title") {
		t.Error("expected title column schema from fillable")
	}
	if !containsEntity(ents, "SCOPE.Schema", "published_at") {
		t.Error("expected published_at column from fillable OR casts")
	}
}

func TestEloquentBelongsToFK(t *testing.T) {
	src := `<?php
class Comment extends Model
{
    public function post(): BelongsTo
    {
        return $this->belongsTo(Post::class);
    }
    public function tags(): BelongsToMany
    {
        return $this->belongsToMany(Tag::class);
    }
}
`
	ents := extract(t, "php_eloquent_orm_data", fi("Comment.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "post") {
		t.Error("expected post belongsTo relation")
	}
	if !containsEntity(ents, "SCOPE.Component", "tags") {
		t.Error("expected tags belongsToMany relation")
	}
}

func TestEloquentMigration(t *testing.T) {
	src := `<?php
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreatePostsTable extends Migration
{
    public function up()
    {
        Schema::create('posts', function (Blueprint $table) {
            $table->id();
            $table->string('title');
            $table->text('content');
            $table->foreign('user_id')->references('id')->on('users');
            $table->timestamps();
        });
    }
}
`
	ents := extract(t, "php_eloquent_orm_data", fi("2023_01_01_create_posts_table.php", "php", src))
	if !containsEntity(ents, "SCOPE.Operation", "create:posts") {
		t.Error("expected create:posts migration entity")
	}
	if !containsEntity(ents, "SCOPE.Schema", "title") {
		t.Error("expected title column schema from blueprint")
	}
	if !containsEntity(ents, "SCOPE.Schema", "fk:user_id") {
		t.Error("expected fk:user_id foreign key entity")
	}
}

func TestEloquentEagerLoading(t *testing.T) {
	src := `<?php
class User extends Model
{
    protected $with = ['profile', 'roles'];
}
`
	ents := extract(t, "php_eloquent_orm_data", fi("User.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "eager_with") {
		t.Error("expected eager_with lazy/eager loading pattern")
	}
}

// ============================================================================
// CycleORM
// ============================================================================

func TestCycleORMEntity(t *testing.T) {
	src := `<?php
namespace App\Entity;

use Cycle\Annotated\Annotation\Entity;
use Cycle\Annotated\Annotation\Column;

#[Entity(table: 'users')]
class User
{
    #[Column(type: 'primary')]
    private int $id;

    #[Column(type: 'string')]
    private string $name;
}
`
	ents := extract(t, "php_cycleorm_data", fi("User.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "User") {
		t.Error("expected User entity model")
	}
	if !containsEntity(ents, "SCOPE.Schema", "id") {
		t.Error("expected id column entity")
	}
	if !containsEntity(ents, "SCOPE.Schema", "name") {
		t.Error("expected name column entity")
	}
}

func TestCycleORMRelation(t *testing.T) {
	src := `<?php
use Cycle\Annotated\Annotation\Relation\HasMany;
use Cycle\Annotated\Annotation\Relation\BelongsTo;

#[Entity]
class Post
{
    #[HasMany(target: Comment::class)]
    private array $comments;

    #[BelongsTo(target: User::class)]
    private User $author;
}
`
	ents := extract(t, "php_cycleorm_data", fi("Post.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "relation:HasMany") {
		t.Error("expected HasMany relation")
	}
	if !containsEntity(ents, "SCOPE.Component", "relation:BelongsTo") {
		t.Error("expected BelongsTo relation")
	}
}

func TestCycleORMQuery(t *testing.T) {
	src := `<?php
class UserRepository
{
    public function findByEmail(string $email): ?User
    {
        return $this->orm->findOne(User::class, ['email' => $email]);
    }
}
`
	ents := extract(t, "php_cycleorm_data", fi("UserRepository.php", "php", src))
	if !containsEntity(ents, "SCOPE.Operation", "query:findOne") {
		t.Error("expected query:findOne entity")
	}
}

// ============================================================================
// Propel ORM
// ============================================================================

func TestPropelTableMap(t *testing.T) {
	src := `<?php
use Propel\Runtime\Map\TableMap;

class UserTableMap extends TableMap
{
    const COL_ID = 'user.id';
    const COL_USERNAME = 'user.username';
    const COL_EMAIL = 'user.email';
}
`
	ents := extract(t, "php_propel_orm_data", fi("UserTableMap.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "UserTableMap") {
		t.Error("expected UserTableMap schema entity")
	}
	if !containsEntity(ents, "SCOPE.Schema", "COL_USERNAME") {
		t.Error("expected COL_USERNAME column entity")
	}
}

func TestPropelRelation(t *testing.T) {
	src := `<?php
class BookTableMap extends TableMap
{
    public function initialize()
    {
        $this->addRelation('Author', AuthorTableMap::CLASS_DEFAULT, RelationMap::MANY_TO_ONE);
        $this->addForeignKey('author_id', 'id', 'INTEGER', 'author', 'id');
    }
}
`
	ents := extract(t, "php_propel_orm_data", fi("BookTableMap.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "relation:Author") {
		t.Error("expected relation:Author component")
	}
	if !containsEntity(ents, "SCOPE.Schema", "foreign_key") {
		t.Error("expected foreign_key entity")
	}
}

func TestPropelQuery(t *testing.T) {
	src := `<?php
$users = UserQuery::create()
    ->filterByIsActive(true)
    ->find();

$book = BookQuery::create()->findOne();
`
	ents := extract(t, "php_propel_orm_data", fi("list.php", "php", src))
	if len(ents) == 0 {
		t.Error("expected Propel query entities from UserQuery::create()")
	}
}

// ============================================================================
// RedBeanPHP
// ============================================================================

func TestRedBeanDispense(t *testing.T) {
	src := `<?php
R::setup('mysql:host=localhost;dbname=shop', 'root', 'password');
$product = R::dispense('product');
$product->name = 'Widget';
$product->price = 9.99;
R::store($product);
`
	ents := extract(t, "php_redbeanphp_data", fi("store.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "product") {
		t.Error("expected product table schema from R::dispense")
	}
}

func TestRedBeanRelation(t *testing.T) {
	src := `<?php
R::associate($product, $category);
$related = R::related($product, 'category');
`
	ents := extract(t, "php_redbeanphp_data", fi("relate.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "relation:associate") {
		t.Error("expected relation:associate component")
	}
	if !containsEntity(ents, "SCOPE.Component", "relation:related") {
		t.Error("expected relation:related component")
	}
}

func TestRedBeanFind(t *testing.T) {
	src := `<?php
$products = R::find('product', ' price > ? ', [10]);
$user = R::findOne('user', ' email = ? ', [$email]);
`
	ents := extract(t, "php_redbeanphp_data", fi("query.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "product") {
		t.Error("expected product schema from R::find")
	}
	if !containsEntity(ents, "SCOPE.Schema", "user") {
		t.Error("expected user schema from R::findOne")
	}
}

func TestRedBeanNoMatch(t *testing.T) {
	src := `<?php class Plain { public function run() {} }`
	ents := extract(t, "php_redbeanphp_data", fi("Plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// PHP SQL Driver Schema (mysql/postgres/sqlite)
// ============================================================================

func TestPHPSQLDriverMySQLCreateTable(t *testing.T) {
	src := `<?php
$pdo = new PDO("mysql:host=localhost;dbname=app", "root", "");
$pdo->exec("
    CREATE TABLE IF NOT EXISTS users (
        id INT AUTO_INCREMENT PRIMARY KEY,
        username VARCHAR(255) NOT NULL,
        email VARCHAR(255) UNIQUE
    )
");
`
	ents := extract(t, "php_sql_driver_schema", fi("setup.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "users") {
		t.Error("expected users table schema")
	}
	if !containsEntity(ents, "SCOPE.Schema", "users.username") {
		t.Error("expected users.username column schema")
	}
}

func TestPHPSQLDriverSQLiteCreateTable(t *testing.T) {
	src := `<?php
$db = new SQLite3('/tmp/test.db');
$db->exec('CREATE TABLE orders (
    id INTEGER PRIMARY KEY,
    total REAL NOT NULL,
    status TEXT DEFAULT "pending"
)');
`
	ents := extract(t, "php_sql_driver_schema", fi("init.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "orders") {
		t.Error("expected orders table schema")
	}
}

func TestPHPSQLDriverNoMatch(t *testing.T) {
	src := `<?php echo "no driver here";`
	ents := extract(t, "php_sql_driver_schema", fi("plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// Behat
// ============================================================================

func TestBehatFeature(t *testing.T) {
	src := `Feature: User login
  In order to access the application
  As a user
  I need to be able to log in

  Scenario: Successful login
    Given I am on the login page
    When I fill in "email" with "user@example.com"
    Then I should see "Dashboard"

  Scenario: Failed login
    Given I am on the login page
    When I fill in "email" with "bad@example.com"
    Then I should see "Invalid credentials"
`
	ents := extract(t, "php_behat_test", fi("login.feature", "gherkin", src))
	if !containsEntity(ents, "SCOPE.Operation", "feature:User login") {
		t.Error("expected feature entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "scenario:Successful login") {
		t.Error("expected Successful login scenario")
	}
	if !containsEntity(ents, "SCOPE.Operation", "scenario:Failed login") {
		t.Error("expected Failed login scenario")
	}
}

func TestBehatContextClass(t *testing.T) {
	src := `<?php
use Behat\Behat\Context\Context;

class FeatureContext implements Context
{
    /**
     * @Given I am on the login page
     */
    #[Given('/^I am on the login page$/')]
    public function iAmOnTheLoginPage()
    {
        // ...
    }

    /**
     * @When I fill in :field with :value
     */
    public function iFillIn($field, $value) {}
}
`
	ents := extract(t, "php_behat_test", fi("FeatureContext.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "FeatureContext") {
		t.Error("expected FeatureContext context class entity")
	}
}

func TestBehatNoMatch(t *testing.T) {
	src := `<?php class NoBehatHere {}`
	ents := extract(t, "php_behat_test", fi("plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// Codeception
// ============================================================================

func TestCodeceptionCest(t *testing.T) {
	src := `<?php
use Codeception\Module\WebDriver;

class UserLoginCest
{
    public function loginSuccessfully(AcceptanceTester $I)
    {
        $I->amOnPage('/login');
        $I->fillField('email', 'user@example.com');
        $I->seeCurrentUrlEquals('/dashboard');
    }

    public function loginWithBadCredentials(AcceptanceTester $I)
    {
        $I->amOnPage('/login');
        $I->seeResponseContains('Invalid');
    }
}
`
	ents := extract(t, "php_codeception_test", fi("UserLoginCest.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "UserLoginCest") {
		t.Error("expected UserLoginCest test suite")
	}
	if !containsEntity(ents, "SCOPE.Operation", "loginSuccessfully") {
		t.Error("expected loginSuccessfully test method")
	}
	if !containsEntity(ents, "SCOPE.Component", "AcceptanceTester") {
		t.Error("expected AcceptanceTester actor")
	}
}

func TestCodeceptionModule(t *testing.T) {
	src := `<?php
use Codeception\Module\Laravel;
use Codeception\Module\WebDriver;

class ApiCest
{
    public function checkEndpoint(FunctionalTester $I)
    {
        $I->sendGET('/api/users');
        $I->seeResponseCodeIs(200);
    }
}
`
	ents := extract(t, "php_codeception_test", fi("ApiCest.php", "php", src))
	if !containsEntity(ents, "SCOPE.Component", "Codeception\\Module\\Laravel") {
		t.Error("expected Laravel module dependency")
	}
}

func TestCodeceptionNoMatch(t *testing.T) {
	src := `<?php class NoCept {}`
	ents := extract(t, "php_codeception_test", fi("plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// Pest
// ============================================================================

func TestPestTestDeclarations(t *testing.T) {
	src := `<?php
uses(Tests\TestCase::class);

it('can create a user', function () {
    $user = User::factory()->create();
    expect($user->id)->toBeInt();
});

test('user email is unique', function () {
    // ...
});

describe('Authentication', function () {
    it('redirects guests to login', function () {
        // ...
    });
});
`
	ents := extract(t, "php_pest_test", fi("UserTest.php", "php", src))
	if !containsEntity(ents, "SCOPE.Operation", "can create a user") {
		t.Error("expected 'can create a user' test case")
	}
	if !containsEntity(ents, "SCOPE.Operation", "user email is unique") {
		t.Error("expected 'user email is unique' test case")
	}
	if !containsEntity(ents, "SCOPE.Component", "describe:Authentication") {
		t.Error("expected Authentication describe block")
	}
	if !containsEntity(ents, "SCOPE.Component", "Tests\\TestCase::class") {
		t.Error("expected uses(Tests\\TestCase::class) dependency")
	}
}

func TestPestDataset(t *testing.T) {
	src := `<?php
dataset('emails', [
    'user@example.com',
    'admin@example.com',
]);

it('validates emails', function (string $email) {
    expect($email)->toBeEmail();
})->with('emails');
`
	ents := extract(t, "php_pest_test", fi("EmailTest.php", "php", src))
	if !containsEntity(ents, "SCOPE.Schema", "dataset:emails") {
		t.Error("expected dataset:emails entity")
	}
}

func TestPestHooks(t *testing.T) {
	src := `<?php
uses(RefreshDatabase::class);

beforeEach(function () {
    $this->user = User::factory()->create();
});

afterEach(function () {
    // cleanup
});
`
	ents := extract(t, "php_pest_test", fi("HookTest.php", "php", src))
	if !containsEntity(ents, "SCOPE.Pattern", "hook:beforeEach") {
		t.Error("expected hook:beforeEach pattern")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "hook:afterEach") {
		t.Error("expected hook:afterEach pattern")
	}
}

func TestPestNoMatch(t *testing.T) {
	src := `<?php echo "no pest here";`
	ents := extract(t, "php_pest_test", fi("plain.php", "php", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
